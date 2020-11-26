#!/bin/bash
docker run -p 8080:80 -v "$(pwd)/mirror:/usr/share/nginx/html" nginx
