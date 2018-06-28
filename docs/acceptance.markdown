# Acceptance

The acceptance environment is located at [this softlayer box](https://control.softlayer.com/devices/details/889955). It's a bosh-lite environment that's deployed manually.

The pipeline is pushing every GREEN release tarball and the accompanying manifest to the acceptance box. All the manifests have no credentials except for the bosh/cf default ones. They are using local storage as fog configuration.

**Note: When delivering a story, include which version of the release contains the fix or feature.**

## Standalone Bits-Service-Release

The release tarball and the accompanying manifest can be found in `/root/bits-service-release/$BITS_VERSION/`. Upload the release, target the manifest and deploy to bosh:

```
bosh upload release /root/bits-service-release/$BITS_VERSION/bits-service-$BITS_VERSION.tgz
bosh deployment /root/bits-service-release/$BITS_VERSION/manifest-$BITS_VERSION.yml
bosh deploy
```

To check the bits-service VM ip:

```
bosh vms bits-service-local
```

## CloudFoundry + Bits-Service-Release

The release tarball and the accompanying manifest can be found in `/root/cf-release/$CF_VERSION/`. Upload the release, target the manifest and deploy to bosh:

```
bosh upload release /root/bits-service-release/$BITS_VERSION/bits-service-$BITS_VERSION.tgz
bosh upload release /root/cf-release/$CF_VERSION/cf-release-$CF_VERSION.tgz
bosh deployment /root/cf-release/$CF_VERSION/manifest-$CF_VERSION.yml
bosh deploy
```

To check the bits-service vm ip:

```
bosh vms cf-warden
```

## Diego
To deploy diego to the acceptance bosh-lite:

```
bosh deployment ~/workspace/bits-service-release/ci/manifests/diego.yml
bosh deploy
```

## Installation
bosh-lite was installed just as described in the [baremetal](baremetal-bosh-lite.markdown) docs.
