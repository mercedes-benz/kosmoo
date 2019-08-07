// SPDX-License-Identifier: MIT
module github.com/daimler/kosmoo

go 1.12

require (
	github.com/gogo/protobuf v1.2.1 // indirect
	github.com/googleapis/gnostic v0.3.0 // indirect
	github.com/gophercloud/gophercloud v0.3.0
	github.com/gopherjs/gopherjs v0.0.0-20190430165422-3e4dfb77656c // indirect
	github.com/imdario/mergo v0.3.7 // indirect
	github.com/kr/pretty v0.1.0 // indirect
	github.com/prometheus/client_golang v1.1.0
	github.com/smartystreets/assertions v1.0.1 // indirect
	github.com/smartystreets/goconvey v0.0.0-20190731233626-505e41936337 // indirect
	github.com/spf13/pflag v1.0.3 // indirect
	golang.org/x/crypto v0.0.0-20190701094942-4def268fd1a4 // indirect
	golang.org/x/net v0.0.0-20190724013045-ca1201d0de80 // indirect
	golang.org/x/oauth2 v0.0.0-20190604053449-0f29369cfe45 // indirect
	golang.org/x/time v0.0.0-20190308202827-9d24e82272b4 // indirect
	google.golang.org/appengine v1.6.1 // indirect
	gopkg.in/check.v1 v1.0.0-20180628173108-788fd7840127 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/ini.v1 v1.44.0
	k8s.io/api v0.0.0
	k8s.io/apimachinery v0.0.0
	k8s.io/client-go v11.0.0+incompatible
	k8s.io/klog v0.3.3
	k8s.io/utils v0.0.0-20190801114015-581e00157fb1 // indirect
)

replace (
	// pinned to kubernetes-1.15.1
	k8s.io/api => k8s.io/api v0.0.0-20190718183219-b59d8169aab5
	k8s.io/apimachinery => k8s.io/apimachinery v0.0.0-20190612205821-1799e75a0719
	k8s.io/client-go => k8s.io/client-go v0.0.0-20190718183610-8e956561bbf5

)
