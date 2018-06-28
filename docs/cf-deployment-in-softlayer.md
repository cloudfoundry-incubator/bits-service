# CF-deployment in Bosh Lite

First install [bosh-cli v2](https://bosh.io/docs/cli-v2.html#install).

## Creating bosh-deployment
See https://bosh.io/docs/bosh-lite.
```bash
mkdir -p ~/workspace/deployments/vbox
cd ~/workspace/deployments/vbox
bosh create-env ~/workspace/bosh-deployment/bosh.yml \
  --state ./state.json \
  -o ~/workspace/bosh-deployment/virtualbox/cpi.yml \
  -o ~/workspace/bosh-deployment/virtualbox/outbound-network.yml \
  -o ~/workspace/bosh-deployment/bosh-lite.yml \
  -o ~/workspace/bosh-deployment/bosh-lite-runc.yml \
  -o ~/workspace/bosh-deployment/jumpbox-user.yml \
  --vars-store ./creds.yml \
  -v director_name="Bosh Lite Director" \
  -v internal_ip=192.168.50.6 \
  -v internal_gw=192.168.50.1 \
  -v internal_cidr=192.168.50.0/24 \
  -v outbound_network_name=NatNetwork
```

```bash
bosh alias-env vbox -e 192.168.50.6 --ca-cert <(bosh int ./creds.yml --path /director_ssl/ca)
bosh -e vbox login --client=admin --client-secret=`bosh int ./creds.yml --path /admin_password`
```

## Upload stemcell
```bash
stemcell_version=$(bosh int ~/workspace/cf-deployment/cf-deployment.yml --path /stemcells/alias=default/version)
bosh -e vbox upload-stemcell https://bosh.io/d/stemcells/bosh-warden-boshlite-ubuntu-trusty-go_agent?v=${stemcell_version}
```

### Update cloud config
```
cd ~/workspace/cf-deployment
bosh -e vbox update-cloud-config bosh-lite/cloud-config.yml
```

## Deploying cf
```bash
bosh -e vbox -d cf deploy cf-deployment.yml \
  -o operations/bosh-lite.yml \
  --vars-store ~/workspace/deployments/vbox/deployment-vars.yml \
  -v system_domain=bosh-lite.com
```

## Test CF
```bash
sudo route add -net 10.244.0.0/16 192.168.50.6
cf api api.bosh-lite.com
cf auth admin $(bosh int ~/workspace/deployments/vbox/deployment-vars.yml --path /cf_admin_password)
```

# CF-deployment in Softlayer
```bash
cd ~/workspace/bits-service-private-config/cf-deployment
```

## Patch /etc/hosts
```bash
sudo sh -c 'echo "10.175.110.153 director.flintstone.ams" >> /etc/hosts'
```

## Alias the director
```bash
bosh alias-env sl -e director.flintstone.ams --ca-cert=~/workspace/bits-service-private-config/certificates/ca.crt
bosh login -e sl --client=admin --client-secret=$(bosh int ~/workspace/bits-service-private-config/bosh.yml --path /jobs/name=bosh/properties/director/user_management/local/users/name=admin/password)
```

## Upload stemcell
```bash
bosh -e sl upload-stemcell https://s3.amazonaws.com/bosh-softlayer-cpi-stemcells/light-bosh-stemcell-3421.11.5-softlayer-xen-ubuntu-trusty-go_agent.tgz
```

## Update cloud config
```bash
bosh -e sl update-cloud-config cloud-config.flintstone-sl.yml
```

## Get a domain name
We registered a dynamic DNS domain name on changeip.com: `cf-deployment.dynamic-dns.net`.
Make sure to set up wildcard domain matching to the router VM, i.e DNS type A record `*.cf-deployment.dynamic-dns.net => 10.175.110.157`.
You can't know the address before the VM is created, so update the DNS after the deployment is done.

## Deploy CF
```bash
bosh2 -e sl -d cf deploy ~/workspace/cf-deployment/cf-deployment.yml \
  --vars-store deployment-vars.yml \
  -v system_domain=cf-deployment.dynamic-dns.net \
  -o ~/workspace/cf-deployment/operations/scale-to-one-az.yml \
  -o ~/workspace/bits-service-ci/operations/stemcell-version.yml \
  -v stemcell_version='3445.7.1' \
  --no-redact
```

## Test
```bash
bosh -e sl -d cf run-errand smoke-tests
```

Make sure that `cf-deployment.dynamic-dns.net` is the first shared domain in cf.
```console
$ cf domains
Getting domains in org test as admin...
name                            status   type
cf-deployment.dynamic-dns.net   shared
```

If it is not, `cf delete-shared-domain <domain>` any older domains.

## Deploy CF with Bits-service enabled
Assuming the previous step worked. Possible BLOBSTORE_TYPE options are `local`, `webdav` and `s3`.

If no bits-service job exists in the deployment, it is necessary to first run the deployment with bits-service disabled by including `-o ~/workspace/bits-service-ci/operations/disable-bits-service.yml` in deployment command. After bits-service VM has bee created and bits-service started, further deployments should succeed.

### Local and webdav
```bash
BLOBSTORE_TYPE=local # or webdav
bosh -e sl -d cf deploy ~/workspace/cf-deployment/cf-deployment.yml \
  --vars-store deployment-vars.yml \
  -v system_domain=cf-deployment.dynamic-dns.net \
  -o ~/workspace/cf-deployment/operations/scale-to-one-az.yml \
  -o ~/workspace/bits-service-ci/operations/stemcell-version.yml \
  -v stemcell_version='3445.7.1' \
  -o ~/workspace/cf-deployment/operations/experimental/bits-service.yml \
  -o ~/workspace/cf-deployment/operations/experimental/bits-service-"${BLOBSTORE_TYPE}".yml \
  --no-redact
```

### S3
```bash
lpass show "Shared-Flintstone"/ci-config --notes > aws-config.yml

BLOBSTORE_TYPE=s3
bosh -e sl -d cf deploy ~/workspace/cf-deployment/cf-deployment.yml \
  --vars-store deployment-vars.yml \
  -v system_domain=cf-deployment.dynamic-dns.net \
  -o ~/workspace/cf-deployment/operations/scale-to-one-az.yml \
  -o ~/workspace/bits-service-ci/operations/stemcell-version.yml \
  -v stemcell_version='3445.7.1' \
  -o ~/workspace/cf-deployment/operations/experimental/bits-service.yml \
  -o ~/workspace/cf-deployment/operations/experimental/bits-service-"${BLOBSTORE_TYPE}".yml \
  -v blobstore_access_key_id=$(bosh int aws-config.yml --path /s3-blobstore-access-key-id) \
  -v blobstore_secret_access_key=$(bosh int aws-config.yml --path /s3-blobstore-secret-access-key) \
  -v aws_region=$(bosh int aws-config.yml --path /s3-blobstore-region) \
  -v resource_directory_key=$(bosh int aws-config.yml --path /s3-blobstore-bucket-name) \
  -v buildpack_directory_key=$(bosh int aws-config.yml --path /s3-blobstore-bucket-name) \
  -v droplet_directory_key=$(bosh int aws-config.yml --path /s3-blobstore-bucket-name) \
  -v app_package_directory_key=$(bosh int aws-config.yml --path /s3-blobstore-bucket-name) \
  --no-redact

rm aws-config.yml
```
