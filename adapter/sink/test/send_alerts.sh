#!/bin/bash
curl -XPOST -H 'Content-type':'application/json' -d @alert.json http://127.0.0.1:8080/alerts
