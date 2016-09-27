# tlswrapd

## Example config
```json
{
	"serverSSH": {
		"bind": "localhost:6621",
		"dial": "example.com:https",
		"protos": ["ssh"]
	},
	"serverVPN": {
		"bind": "localhost:3000",
		"dial": "example.com:https",
		"protos": ["openvpn"]
	}
}
```

TODO:
-----
- [ ] Tests
- [ ] Docs
