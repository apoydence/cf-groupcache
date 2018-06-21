# cf-groupcache
Implements a HTTP PeerPicker for GroupCache that is compatible with Cloud Foundry.

CF Groupcache enables a Cloud Foundry application to use
[groupcache](https://github.com/golang/groupcache). It marks each request with
the according `X-CF-APP-INSTANCE` so the
[Gorouter](https://github.com/cloudfoundry/gorouter) can route each request
correctly.
