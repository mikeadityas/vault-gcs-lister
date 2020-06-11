# Vault GCS Lister
Vault GCS Lister is a dummy service to periodically list GCS buckets using dyanmic
service account key from a modified Hashicorp Vault's GCP secrets engine that has
some kind of caching mechanism to overcome the limit of 10 service account key per
roleset.

The purpose of this dummy service is to test the stability of the cache.

## Build
```
make build
```

## Flags
`--secrets-path`:  
GCP secrets engine path

`--project-id`:  
GCP project ID

`--interval`:  
The interval to list the GCS bucket

`--early-renewal`:  
The early renewal duration

`--vault.address`:  
Vault address

`--vault.role`:  
Vault rolename

`--log.level`:  
Log level (debug, info,warning, error)

`--log.format`:  
Log format (text, json)

`--tls.ca`:  
Location of CAcert file

`--tls.cert`:  
Location of cert file

`--tls.key`:  
Location of key file