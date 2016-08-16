# tlswrapd

## Example config
```toml
[[proxies]]
name      = "example1SSH"
bind      = "localhost:6621"
dial      = "example1.com:443"
protocols = ["ssh"]
[[proxies]]
name      = "example2SSH"
bind      = "localhost:6622"
dial      = "example2.com:443"
protocols = ["ssh"]
```
