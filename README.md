# afro
Afro is an API Client whose core philosophy is chaining requests together.

## Stack
It is written in Golang.
The configuration and data is all saved to files on the local filesystem.

## How it works
Basically Afro lets you make HTTP requests from the command line and save the request setup as well replay it or use its response as part of another.

### Bundles
Afro allows you define bundles. A bundle is a collection of related requests. They may share things such as the same base URL.

By default, if the current working directory has an afro config file, it is considered a bundle and actions will use the configuration defined. Alternatively, you can also pass in a `--bundle` arg to specify what bundle to use for the command.

You can run `afro init` to set up a new bundle. It will interactively walk through questions like base_url and any common headers and save these so that they are always applied to requests in the bundle.

### Making requests
To make a request simply call afro along with the HTTP verb and the URL. If you pass in a relative path, ie without a scheme, then Afro will automatically prepend the base URL to yours along with sending anything else configured such as common headers.

An example GET request would be `afro get https://api.etin.dev`

We can optionally pass a request body with `-b` or `--body`. This argument will either take in a string, denoted by quotes or a path to a file which will be treated as the body.

Headers can also be optionally passed with `-h` or `--header` or `--headers`. Headers are specified as a string, denoted by quotation marks in the form `-h "Accepts: application/json"`. Multiple headers can be specified by separating each entry with a semi colon like so `-h "Accepts: application/json; Content-Type: application/json"`.

If the same header is set on the bundle and in the request, the request takes precedence.

To opt out of any default configuration for this specify request, use the argument `--no-headers`.

### Saving requests
Afro allows you save requests so that they can be easily called again. A request can be saved by making the request in the regular way along with a `--save="my-request-name"` option.

### Chaining requests, extracting vars, and branching
You can create a chain in Afro, which is a set of linked requests that run in order.

You can configure chains in your `afro.yaml`.

#### Extraction
Extraction happens via JSON path and will store the extracted value in the named variable. That named variable can then be used in the next request as needed by using `{{var_name}}` syntax in URL, body, or headers.


#### Dynamic Variables
Afro supports built-in dynamic variables that are evaluated at runtime:
- `{{$timestamp}}`: Current Unix timestamp.
- `{{$uuid}}`: A random UUID-like string.

#### Branching
You can specify branching logic based on the status code of a response. This allows you to implement flows like "if 401, login, then retry".

#### Assertions
You can verify response data using assertions. If an assertion fails, the chain stops.
```yaml
- request: "get_members"
  extract:
    count: "$.length()"
  assert:
    - left: "{{count}}"
      op: ">="
      right: "1"
```
Supported operators: `==`, `!=`, `>`, `>=`, `<`, `<=`.

Example chain configuration in `afro.yaml`:

```yaml
chains:
  get_data_flow:
    - request: "get_data"
      on_status:
        401:
          - request: "login"
            extract:
              token: "$.token"
          - request: "get_data" # Retry with new token
```



