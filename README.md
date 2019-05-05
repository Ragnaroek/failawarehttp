[![Build Status](https://travis-ci.org/Ragnaroek/failawarehttp.svg?branch=master)](https://travis-ci.org/Ragnaroek/failawarehttp)
[![Coverage Status](https://coveralls.io/repos/github/Ragnaroek/failawarehttp/badge.svg?branch=master)](https://coveralls.io/github/Ragnaroek/failawarehttp?branch=master)

# failawarehttp
Go http client, with an awareness for failures

Features
* zero/very few dependencies, plain Go
* does sane things out of the box (meaning: exponential backoff with jitter)
* easy code
* drop in replacement for net/http client (not there yet, currently a subset)
