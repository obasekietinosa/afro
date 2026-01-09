# afro
Afro is an API Client whose core philosophy is chaining requests together.

## Stack
It is written in Golang.

## How it works
Basically Afro lets you make HTTP requests from the command line and save the request setup as well replay it or use its response as part of another.

### Bundles
Afro allows you define bundles. A bundle is a collection of related requests. They may share things such as the same base URL or a common auth scheme.

By default, the current working directory is considered a bundle and actions performed don't need to be specified. Alternatively, you can also pass in a `--bundle` arg to specify what bundle to use for the command.

You can run `afro init` to set up a new bundle

### Making requests
To make a request simply call afro along with the HTTP verb and the URL. 
