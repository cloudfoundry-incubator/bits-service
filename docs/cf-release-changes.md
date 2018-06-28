# Summary of cf-release manifest changes

## cf-release manifest - without flag

* Director uuid
```
director_uuid: <%= `bosh status --uuid` %>
```

* Have faster CATs
```
properties:
  acceptance_tests:
    nodes: 1 --> 4
```

## cf-release manifest - with flag

* all the above

* Add bits-service job

```
jobs:
- instances: 1
  name: bits_service_z1
  networks:
  - name: cf1
    static_ips:
    - 10.244.0.74
  persistent_disk: 6064
  resource_pool: small_z1
  templates:
  - name: bits-service
    release: bits-service
  update:
    max_in_flight: 1
    serial: true
```

* Co-locate bits-service release

```
releases:
- name: cf
  version: latest
- name: bits-service
  version: <%= ENV.fetch('RELEASE_VERSION', 'latest') %>
```

* Co-locate bits-service release (again):
```
  meta:
  environment: cf-warden
  releases:
  - name: cf
    version: latest
  - name: bits-service
    version: <%= ENV.fetch('RELEASE_VERSION', 'latest') %>
```

* Have properties merged in from our templates
```
properties:
  bits-service:
    <<: (( merge ))
```

* Enable bits-service in Cloud Controller

```
cc:
-    bits_service:
-      enabled: true
-      endpoint: http://10.244.0.74
```

And merge with template from bits-service-release. See the CI task for generating the cf-with-flag.yml
