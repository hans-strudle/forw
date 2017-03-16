# forw
## splits HTTP requests to multiple targets

This is a clone of teeproxy (https://github.com/chrislusf/teeproxy)

You only need to write a simple `config.json` file, which defines which port forw listens on, which endpoint the request should proxy to, and which endpoints a copy of the request should be sent to.

## Usage:
```bash
> ./forw -h
Usage of ./forw:
  -d=false: Debug messages
  -f="config.json": Path to config file

```


example `config.json`:
```json
{
	"listen": ":8080",
	"proxy": ":8000",
	"forwards": [
		":3000",
		"10.0.100.123:4000",
		"webapp.com:80"
	]
}
```

The process will listen for a SIGHUP signal, and reload the JSON, meaning there is no need to stop/start the server. You can simply change the `config.json` and send a SIGHUP to the process.
```bash
ps aux | grep forw # get process pid
kill -1 PID # send SIGHUP to process, which reloads config
```


