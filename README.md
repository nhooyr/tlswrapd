# tlswrapd

## Example config
```json
{
	"serverSSH": {
		"bind": "localhost:6621",
		"dial": "example.com:https",
		"proto": "ssh"
	},
	"serverVPN": {
		"bind": "localhost:3000",
		"dial": "example.com:https",
		"proto": "openvpn"
	}
}
```

TODO:
-----
- [ ] Tests
- [ ] Docs
