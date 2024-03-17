// SPDX-License-Identifier: MIT

package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/gophercloud/gophercloud"
	"github.com/gophercloud/gophercloud/openstack"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"gopkg.in/ini.v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"

	"github.com/mercedes-benz/kosmoo/pkg/metrics"
)

var (
	refreshInterval = flag.Int64("refresh-interval", 120, "Interval between scrapes to OpenStack API (default 120s)")
	addr            = flag.String("addr", ":9183", "Address to listen on")
	cloudConfFile   = flag.String("cloud-conf", "", "path to the cloud.conf file. If this path is not set the scraper will use the usual OpenStack environment variables.")
	kubeconfig      = flag.String("kubeconfig", os.Getenv("KUBECONFIG"), "Path to the kubeconfig file to use for CLI requests. (uses in-cluster config if empty)")
	metricsPrefix   = flag.String("metrics-prefix", metrics.DefaultMetricsPrefix, "Prefix used for all metrics")
)

var (
	scrapeDuration *prometheus.GaugeVec
	scrapedAt      *prometheus.GaugeVec
	scrapedStatus  *prometheus.GaugeVec
)

var (
	clientset       *kubernetes.Clientset
	backoffSleep    = time.Second
	maxBackoffSleep = time.Hour
)

func registerMetrics(prefix string) {
	scrapeDuration = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: metrics.AddPrefix("scrape_duration", prefix),
			Help: "Time in seconds needed for the last scrape",
		},
		[]string{"refresh_interval"},
	)
	scrapedAt = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: metrics.AddPrefix("scraped_at", prefix),
			Help: "Timestamp when last scrape started",
		},
		[]string{"refresh_interval"},
	)
	scrapedStatus = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: metrics.AddPrefix("scrape_status_succeeded", prefix),
			Help: "Scrape status succeeded",
		},
		[]string{"refresh_interval"},
	)

	prometheus.MustRegister(scrapeDuration)
	prometheus.MustRegister(scrapedAt)
	prometheus.MustRegister(scrapedStatus)
}

// metrixMutex locks to prevent race-conditions between scraping the metrics
// endpoint and updating the metrics
var metricsMutex sync.Mutex

func main() {
	klog.InitFlags(nil)
	flag.Parse()

	klog.Infof("starting kosmoo at %s", *addr)

	registerMetrics(*metricsPrefix)
	metrics.RegisterMetrics(*metricsPrefix)

	// start prometheus metrics endpoint
	go func() {
		// klog.Info().Str("addr", *addr).Msg("starting prometheus http endpoint")
		klog.Infof("starting prometheus http endpoint at %s", *addr)
		metricsMux := http.NewServeMux()
		promHandler := promhttp.Handler()
		metricsMux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
			// metricsMutex ensures that we do not drop the metrics during promHandler.ServeHTTP call
			metricsMutex.Lock()
			defer metricsMutex.Unlock()
			promHandler.ServeHTTP(w, r)
		})

		metricsMux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			if _, err := w.Write([]byte(http.StatusText(http.StatusOK))); err != nil {
				klog.Warningf("error handling /healthz: %v", err)
			}
		})

		err := http.ListenAndServe(*addr, metricsMux)
		klog.Fatalf("prometheus http.ListenAndServe failed: %v", err)
	}()

	for {
		if run() != nil {
			klog.Errorf("error during run - sleeping %s", backoffSleep)
			time.Sleep(backoffSleep)
			backoffSleep = min(2*backoffSleep, maxBackoffSleep)
		}
	}
}

// run does the initialization of the operational exporter and also the metrics scraping
func run() error {
	var authOpts gophercloud.AuthOptions
	var endpointOpts gophercloud.EndpointOpts
	var err error

	// get OpenStack credentials
	if *cloudConfFile != "" {
		authOpts, endpointOpts, err = authOptsFromCloudConf(*cloudConfFile)
		if err != nil {
			return logError("unable to read OpenStack credentials from cloud.conf: %v", err)
		}
		klog.Infof("OpenStack credentials read from cloud.conf file at %s", *cloudConfFile)
	} else {
		authOpts, err = openstack.AuthOptionsFromEnv()
		if err != nil {
			return logError("unable to get authentication credentials from environment: %v", err)
		}
		endpointOpts = gophercloud.EndpointOpts{
			Region: os.Getenv("OS_REGION_NAME"),
		}
		if endpointOpts.Region == "" {
			endpointOpts.Region = "nova"
		}
		klog.Info("OpenStack credentials read from environment")
	}

	// get kubernetes clientset
	var config *rest.Config
	config, err = clientcmd.BuildConfigFromFlags("", *kubeconfig)
	if err != nil {
		return logError("unable to get kubernetes config: %v", err)
	}
	clientset, err = kubernetes.NewForConfig(config)
	if err != nil {
		return logError("error creating kubernetes Clientset: %v", err)
	}

	// authenticate to OpenStack
	provider, err := openstack.AuthenticatedClient(authOpts)
	if err != nil {
		return logError("unable to authenticate to OpenStack: %v", err)
	}
	_ = provider

	klog.Infof("OpenStack authentication was successful: username=%s, tenant-id=%s, tenant-name=%s", authOpts.Username, authOpts.TenantID, authOpts.TenantName)

	// start scraping loop
	for {
		err := updateMetrics(provider, endpointOpts, clientset, authOpts.TenantID)
		if err != nil {
			return err
		}
		time.Sleep(time.Second * time.Duration(*refreshInterval))
	}
}

// original struct is in
// "github.com/kubernetes/kubernetes/pkg/cloudprovider/providers/openstack"
// and read by
// "gopkg.in/gcfg.v1"
// maybe use that to be type-safe

// For Reference, the layout of the [Global] section in cloud.conf
// 	Global struct {
// 		AuthURL    string `gcfg:"auth-url"`
// 		Username   string
// 		UserID     string `gcfg:"user-id"`
// 		Password   string
// 		TenantID   string `gcfg:"tenant-id"`
// 		TenantName string `gcfg:"tenant-name"`
// 		TrustID    string `gcfg:"trust-id"`
// 		DomainID   string `gcfg:"domain-id"`
// 		DomainName string `gcfg:"domain-name"`
// 		Region     string
// 		CAFile     string `gcfg:"ca-file"`
// 	}

// authOptsFromCloudConf reads the cloud.conf from `path` and returns the read AuthOptions
func authOptsFromCloudConf(path string) (gophercloud.AuthOptions, gophercloud.EndpointOpts, error) {
	cfg, err := ini.Load(path)
	if err != nil {
		return gophercloud.AuthOptions{}, gophercloud.EndpointOpts{}, fmt.Errorf("unable to read cloud.conf content: %v", err)
	}

	global, err := cfg.GetSection("Global")
	if err != nil {
		return gophercloud.AuthOptions{}, gophercloud.EndpointOpts{}, fmt.Errorf("unable get Global section: %v", err)
	}

	ao := gophercloud.AuthOptions{
		IdentityEndpoint: global.Key("auth-url").String(),
		Username:         global.Key("username").String(),
		UserID:           global.Key("user-id").String(),
		Password:         global.Key("password").String(),
		DomainID:         global.Key("domain-id").String(),
		DomainName:       global.Key("domain-name").String(),
		TenantID:         global.Key("tenant-id").String(),
		TenantName:       global.Key("tenant-name").String(),
		AllowReauth:      true,
	}
	eo := gophercloud.EndpointOpts{
		Region: global.Key("region").String(),
	}

	return ao, eo, nil
}

func updateMetrics(provider *gophercloud.ProviderClient, eo gophercloud.EndpointOpts, clientset *kubernetes.Clientset, tenantID string) error {
	metricsMutex.Lock()
	defer metricsMutex.Unlock()

	var errs []error
	scrapeStart := time.Now()

	cinderClient, neutronClient, loadbalancerClient, computeClient, err := getClients(provider, eo)
	if err != nil {
		err := logError("creating openstack clients failed: %v", err)
		errs = append(errs, err)
	} else {
		if err := metrics.PublishCinderMetrics(cinderClient, clientset, tenantID); err != nil {
			err := logError("scraping cinder metrics failed: %v", err)
			errs = append(errs, err)
		}

		if err := metrics.PublishNeutronMetrics(neutronClient, tenantID); err != nil {
			err := logError("scraping neutron metrics failed: %v", err)
			errs = append(errs, err)
		}

		if err := metrics.PublishLoadBalancerMetrics(loadbalancerClient, tenantID); err != nil {
			err := logError("scraping load balancer metrics failed: %v", err)
			errs = append(errs, err)
		}

		if err := metrics.PublishListenerMetrics(loadbalancerClient, tenantID); err != nil {
			err := logError("scraping listener metrics failed: %v", err)
			errs = append(errs, err)
		}

		if err := metrics.PublishServerMetrics(computeClient, tenantID); err != nil {
			err := logError("scraping server metrics failed: %v", err)
			errs = append(errs, err)
		}

		if err := metrics.PublishFirewallV1Metrics(neutronClient, tenantID); err != nil {
			err := logError("scraping firewall v1 metrics failed: %v", err)
			errs = append(errs, err)
		}

		if err := metrics.PublishFirewallV2Metrics(neutronClient, tenantID); err != nil {
			err := logError("scraping firewall v2 metrics failed: %v", err)
			errs = append(errs, err)
		}
	}

	duration := time.Since(scrapeStart)
	scrapeDuration.WithLabelValues(
		fmt.Sprintf("%d", *refreshInterval),
	).Set(duration.Seconds())

	scrapedAt.WithLabelValues(
		fmt.Sprintf("%d", *refreshInterval),
	).Set(float64(scrapeStart.Unix()))

	if len(errs) > 0 {
		scrapedStatus.WithLabelValues(
			fmt.Sprintf("%d", *refreshInterval),
		).Set(0)
		return fmt.Errorf("errors during scrape loop")
	}

	scrapedStatus.WithLabelValues(
		fmt.Sprintf("%d", *refreshInterval),
	).Set(1)

	// reset backoff after successful scrape
	backoffSleep = time.Second

	return nil
}

func getClients(provider *gophercloud.ProviderClient, endpointOpts gophercloud.EndpointOpts) (cinder, neutron, loadbalancer, compute *gophercloud.ServiceClient, err error) {
	cinderClient, err := openstack.NewBlockStorageV3(provider, endpointOpts)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("unable to get cinder client: %v", err)
	}

	neutronClient, err := openstack.NewNetworkV2(provider, endpointOpts)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("unable to get neutron client: %v", err)
	}

	computeClient, err := openstack.NewComputeV2(provider, endpointOpts)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("unable to get compute client: %v", err)
	}

	if _, err := provider.EndpointLocator(gophercloud.EndpointOpts{Type: "load-balancer", Availability: gophercloud.AvailabilityPublic}); err != nil {
		// we can use the neutron client to access lbaas because no octavia is available
		return cinderClient, neutronClient, neutronClient, computeClient, nil
	}

	loadbalancerClient, err := openstack.NewLoadBalancerV2(provider, endpointOpts)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("failed to create loadbalancer service client: %v", err)
	}
	return cinderClient, neutronClient, loadbalancerClient, computeClient, nil
}

func min(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}

func logError(format string, a ...interface{}) error {
	err := fmt.Errorf(format, a...)
	klog.Error(err)
	return err
}
