# correios

Track your orders from [correios](http://www.correios.com.br/) via command line.

# Usage

```
correios check [code1] [code2]
correios list
correios add <code> [name]
correios rm <code || name>
```

The codes are stored as a YAML file in `$HOME/.correios.yml` by default, use
the `-f` or `--filestore` flag to pick another filestore.

# Author

Murilo Santana <<mvrilo@gmail.com>>

# License

MIT
