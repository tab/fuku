#!/bin/sh
# a typo in the shared readiness anchor breaks the config
sed -i "" "s/  type: http$/  type: htttp/" fuku.yaml
