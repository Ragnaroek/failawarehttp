[![Build Status](https://travis-ci.org/Ragnaroek/failawarehttp.svg?branch=master)](https://travis-ci.org/Ragnaroek/failawarehttp)

# failawarehttp
Go http client, with an awareness for failures

Goals
* zero dependencies, plain Go
* does sane things out of the box (meaning: exponential backoff with jitter)
* easy code
