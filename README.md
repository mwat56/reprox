# ReProx

[![golang](https://img.shields.io/badge/Language-Go-green.svg)](https://golang.org/)
[![GoDoc](https://godoc.org/github.com/mwat56/reprox?status.svg)](https://godoc.org/github.com/mwat56/reprox)
[![Go Report](https://goreportcard.com/badge/github.com/mwat56/reprox)](https://goreportcard.com/report/github.com/mwat56/reprox)
[![Issues](https://img.shields.io/github/issues/mwat56/reprox.svg)](https://github.com/mwat56/reprox/issues?q=is%3Aopen+is%3Aissue)
[![Size](https://img.shields.io/github/repo-size/mwat56/reprox.svg)](https://github.com/mwat56/reprox/)
[![Tag](https://img.shields.io/github/tag/mwat56/reprox.svg)](https://github.com/mwat56/reprox/tags)
[![View examples](https://img.shields.io/badge/learn%20by-examples-0077b3.svg)](https://github.com/mwat56/reprox/blob/main/_demo/demo.go)
[![License](https://img.shields.io/github/mwat56/reprox.svg)](https://github.com/mwat56/reprox/blob/main/LICENSE)

<!-- TOC -->

- [ReProx](#reprox)
	- [Purpose](#purpose)
	- [Installation](#installation)
	- [Usage](#usage)
		- [Configuration](#configuration)
		- [Running](#running)
	- [Libraries](#libraries)
	- [Licence](#licence)

<!-- /TOC -->

----

## Purpose

While developing certain applications, it was necessary for me to have a reverse proxy server that can route requests to different backend servers based on the hostname in the request. While this required some CNAME setup in my domain's public configuration it allowed me to call my applications during development from the outside using different hostnames which in turn pointed to different internal backend servers.

`reprox` is that hostname-based reverse proxy server written in `Go`. It is designed to act as a reverse proxy, forwarding requests to the appropriate backend servers based on the hostname in the request.

As always, my goal was twofold: (a) to learn new stuff and (b) to create a tool that I can use for my own needs.

## Installation

You can use `Go` to install this package for you. This will install the `reprox` binary in your `$GOPATH/bin` directory.

    go install github.com/mwat56/reprox@latest

## Usage

First of all, because `reprox` needs to bind to the privileged ports 80 and 443, it must be run with `root` privileges.

If you're running a GNU/Linux system using `systemd`, you can use the following service file to start the reverse proxy server:

	[Unit]
	Description=Hostname-based Reverse Proxy
	Documentation=https://github.com/mwat56/reprox/
	After=network.target

	[Service]
	Type=simple
	User=root
	Group=root
	WorkingDirectory=/tmp
	ExecStart=/opt/bin/reprox
	Restart=on-failure
	RestartSec=1s

	[Install]
	WantedBy=multi-user.target
	Alias=reprox-server.service

Depending on the directory you chose for storing the `reprox` binary, you may need to adjust the `ExecStart` line in the service file.

### Configuration

The configuration file is a JSON file – expected in `/etc/reprox/reprox.json` – with the following structure:

	{
		// Map of hostnames to their backend server URLs
		"hosts": {
			"api.example.com": "http://10.0.0.101:8080",    // API server backend
			"app.example.com": "http://10.0.0.102:3000"     // Application server backend
		},
		"access_log": "./access.log",      // Path to access log file
		"error_log": "./error.log",        // Path to error log file
		"tls_cert": "/etc/ssl/certs/reprox.pem",      // Path to TLS certificate
		"tls_key": "/etc/ssl/private/reprox.key",     // Path to TLS private key
		"max_requests": 100,   // Maximum number of requests allowed per window
		"window_size": 60      // Time window in seconds for rate limiting
	}

**JSON configuration structure:**

1. `hosts`: A map/dictionary where:
	- Keys are host names (e.g., api.example.com) by which this server is contacted.
	- Values are backend server URLs (e.g., `http://10.0.0.101:8080`). This allows the proxy to route requests based on the hostname to different backend servers.
2. Logging configuration:
	- `access_log`: Path where successful requests are logged.
	- `error_log`: Path where errors and issues are logged.
3. TLS/HTTPS configuration:
	- `tls_cert`: Path to the SSL/TLS certificate file.
	- `tls_key`: Path to the private key file These enable HTTPS support for secure connections.
4. Rate limiting settings:
	- `max_requests`: Maximum number of requests allowed (100 in this example).
	- `window_size`: Time period in seconds (60 in this example) for the rate limit Together these create a rate limit of 100 requests per minute per client.

This configuration allows the proxy to:

- Route traffic to different backend servers based on host/domain name,
- support HTTPS,
- log access and errors,
- protect backend servers from overload through rate limiting.

### Running

To run the reverse proxy server, execute the following command:

	sudo /opt/bin/reprox &

This will start the server in the background.

Alternatively, you can use `systemd` to start the server as a service.

	sudo systemctl enable reprox-server.service  # only if not done before
	sudo systemctl start reprox-server.service   # start the service
	sudo systemctl status reprox-server.service  # check the service status

## Libraries

The following external libraries were used building `reprox`:

* [ApacheLogger](https://github.com/mwat56/apachelogger)
* [RateLimit](https://github.com/mwat56/ratelimit)

## Licence

	Copyright © 2024, 2025  M.Watermann, 10247 Berlin, Germany
		    All rights reserved
		EMail : <support@mwat.de>

> This program is free software; you can redistribute it and/or modify it under the terms of the GNU General Public License as published by the Free Software Foundation; either version 3 of the License, or (at your option) any later version.
>
> This software is distributed in the hope that it will be useful, but WITHOUT ANY WARRANTY; without even the implied warranty of MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.
>
> You should have received a copy of the GNU General Public License along with this program. If not, see the [GNU General Public License](http://www.gnu.org/licenses/gpl.html) for details.

----
[![GFDL](https://www.gnu.org/graphics/gfdl-logo-tiny.png)](http://www.gnu.org/copyleft/fdl.html)
