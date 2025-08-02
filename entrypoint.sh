#!/bin/sh
# write the SA key to a file that only this container can read
echo "$SA_JSON" > /tmp/adc.json
chmod 400 /tmp/adc.json
export GOOGLE_APPLICATION_CREDENTIALS=/tmp/adc.json
exec "$@"
