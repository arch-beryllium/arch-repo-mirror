#!/bin/bash
cd mirror || exit
python3 -m http.server 8080
