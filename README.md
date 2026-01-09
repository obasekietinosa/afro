# afro
Afro is an API Client whose core philosophy is chaining requests together.

## Stack
It is written in Golang.
The configuration and data is all saved to files on the local filesystem.

## How it works
Basically Afro lets you make HTTP requests from the command line and save the request setup as well replay it or use its response as part of another.

### Bundles
Afro allows you define bundles. A bundle is a collection of related requests. They may share things such as the same base URL or a common auth scheme.

By default, if the current working directory has an afro config file, it is considered a bundle and actions will use the configuration defined. Alternatively, you can also pass in a `--bundle` arg to specify what bundle to use for the command.

You can run `afro init` to set up a new bundle. It will interactively walk through questions like base_url, authentication schemes, any common headers and save these so that they are always applied to requests in the bundle.

### Authentication
Afro offers support for authentication schemes and if one is presented will automatically retry it in requests if it receives an auth error.

For username and password authentication, we can specify username and password in the bundle configuration and have that saved authentication be used.

For example, you could have `afro auth --username=etin --password --save="login-as-etin"` (which will ask for password via the system prompt) and then in the future simply run `afro auth login-as-etin`

### Making requests
To make a request simply call afro along with the HTTP verb and the URL. If you pass in a relative path, ie without a scheme, then Afro will automatically prepend the base URL to yours along with sending anythibg else configured such as authentication and headers.

An example GET request would be `afro get https://api.etin.dev`

We can optionally pass a request body with `-b` or `--body`. This argument will either take in a string, denoted by quotes or a path to a file which will be treated as the body.

Headers can also be optionally passed with `-h` or `--header` or `--headers`. Headers are specified as a string, denoted by wuotation marks in the form `-h "Accepts: application/json"`. Multiple headers can be specified by separating each entry with a semi colon like so `-h "Accepts: application/json; Content-Type: application/json"`.

If the same header is set on the bundle and in the request, the request takes precedence.

To opt out of any default configuration for this specify request, use the argument `--no-auth` or `--no-headers` as required. You can opt out of headers and still pass specific headers to the request.

