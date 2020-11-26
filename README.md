# arch-repo-mirror

A tool to create a local mirror for https://github.com/jld3103/arch-linux-on-beryllium

## Usage

```bash
go run main.go
```

This tool doesn't use parallel downloads, because some mirrors seem to dislike it, but once you mirrored everything,
updating it should be fast.

## Serving the mirror

Use the `serve.sh` script to host the mirror on `http://localhost:8080`.  
Then put `http://your.ip.address:8080/$repo/$arch` as your server URL in the pacman config.