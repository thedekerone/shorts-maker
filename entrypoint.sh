#!/bin/sh
echo "$WI_CONFIG" > /tmp/wi-config.json
exec "$@"
