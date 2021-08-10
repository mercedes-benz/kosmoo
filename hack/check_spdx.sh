#!/bin/bash
# SPDX-License-Identifier: MIT

set -e

array_contains() {
  local search="$1"
  local element
  shift
  for element; do
    if [[ "${element}" == "${search}" ]]; then
      return 0
     fi
  done
  return 1
}

HEADER="SPDX-License-Identifier: MIT"
HEADER_WHITELIST=("./go.sum" "./LICENSE" "./kosmoo" "./logo/logo.png")

all_files=()
export IFS=$'\n'
while IFS='' read -r line; do all_error_files+=("$line"); done < <(find . -type f -not -path './.git*' -not -path './vendor/*' -not -path './tmp/*' -not -path './.idea/*')
unset IFS

errors=()
for file in "${all_error_files[@]}"; do
  array_contains "$file" "${HEADER_WHITELIST[@]}" && in_whitelist=$? || in_whitelist=$?
  if [[ "${in_whitelist}" -eq "0" ]]; then
    continue
  fi
  set +e
  matches=$(head -n 2 $file | grep "$HEADER" | wc -l)
  set -e
  if [[ "$matches" -ne "1" ]]; then
    errors+=( "${file}" )
    echo "error checking ${file} for the SPDX header"
  fi
done

if [ ${#errors[@]} -eq 0 ]; then
  echo 'Congratulations! All source files have been checked for SPDX header.'
else
    echo
    echo 'Please review the above files. They seem to miss the following header as comment:'
    echo "$HEADER"
    echo
    exit 1
fi
