## Tests with Toxiproxy


### Prerequisites

* docker, docker-compose should be installed

### Setup

```shell
$ docker-compose up -d
```

### Run

```shell
$ go run ./
```

### Test

```shell
$ go test -v .
$ go test -v . -run TestMultipleToxics
```


### References

* [Technitium API docs](https://github.com/TechnitiumSoftware/DnsServer/blob/master/APIDOCS.md)
