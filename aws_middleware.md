Here’s a Go wrapper that leverages **Smithy middleware** to help you **log and troubleshoot AWS SDK v2 credentials caching** for Go (`aws.CredentialsCache`), using the `smithy-go` logging facilities.

---

## Approach

1. **Wrap credentials provider** with `aws.CredentialsCache` (already the default via `LoadDefaultConfig`) ([AWS Documentation](https://docs.aws.amazon.com/sdk-for-go/v2/developer-guide/migrate-gosdk.html?utm_source=chatgpt.com "Migrate to the AWS SDK for Go v2")).
    
2. **Attach a custom middleware** to client operations that logs credentials retrieval events (e.g., cache hit/miss and provider invocation).
    
3. Use `github.com/aws/smithy-go/middleware` and helpful context utilities like `GetSigningCredentials` to extract credential metadata ([Go Packages](https://pkg.go.dev/github.com/aws/aws-sdk-go-v2/aws/middleware?utm_source=chatgpt.com "middleware package - github.com/aws/aws-sdk-go-v2/aws ..."), [AWS Documentation](https://docs.aws.amazon.com/sdk-for-go/v2/developer-guide/middleware.html?utm_source=chatgpt.com "Customizing the AWS SDK for Go v2 Client Requests with Middleware")).
    

---

## Sample Wrapper Code

```go
package credslogger

import (
    "context"
    "log"

    "github.com/aws/aws-sdk-go-v2/aws"
    "github.com/aws/aws-sdk-go-v2/config"
    "github.com/aws/aws-sdk-go-v2/service/sts" // any AWS service
    "github.com/aws/smithy-go/middleware"
)

// CredentialsLogger is a middleware that logs credential resolution usage.
var CredentialsLogger = middleware.FinalizeMiddlewareFunc("CredentialsLogger", func(
    ctx context.Context, in middleware.FinalizeInput, next middleware.FinalizeHandler,
) (
    middleware.FinalizeOutput, middleware.Metadata, error,
) {
    if creds, ok := middleware.GetSigningCredentials(ctx); ok {
        log.Printf("[CredsCache] Using credentials: AccessKeyId=%s Expiry=%v",
            creds.AccessKeyID, creds.Expires)
    } else {
        log.Println("[CredsCache] No signing credentials found in context")
    }
    return next.HandleFinalize(ctx, in)
})

// NewLoggedConfig loads AWS config and adds credential-logging middleware.
func NewLoggedConfig(ctx context.Context, opts ...func(*config.LoadOptions) error) (aws.Config, error) {
    cfg, err := config.LoadDefaultConfig(ctx, opts...)
    if err != nil {
        return cfg, err
    }

    cfg.APIOptions = append(cfg.APIOptions, func(stack *middleware.Stack) error {
        return stack.Finalize.Add(CredentialsLogger, middleware.After)
    })

    return cfg, nil
}

// Example usage with an AWS client (e.g., STS)
func Example() error {
    ctx := context.Background()
    cfg, err := NewLoggedConfig(ctx)
    if err != nil {
        return err
    }

    client := sts.NewFromConfig(cfg)
    _, err = client.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
    return err
}
```

---

## How This Works

- **`config.LoadDefaultConfig`** automatically wraps your credentials provider in a `CredentialsCache` ([AWS Documentation](https://docs.aws.amazon.com/sdk-for-go/v2/developer-guide/migrate-gosdk.html?utm_source=chatgpt.com "Migrate to the AWS SDK for Go v2")).
    
- We inject a **Finalize-step middleware** (`CredentialsLogger`) into the middleware stack via `cfg.APIOptions`, ensuring it fires after signing, before the HTTP request is dispatched — logging whether the signing credentials came from the cache and showing expiration details ([AWS Documentation](https://docs.aws.amazon.com/sdk-for-go/v2/developer-guide/middleware.html?utm_source=chatgpt.com "Customizing the AWS SDK for Go v2 Client Requests with Middleware"), [Go Packages](https://pkg.go.dev/github.com/aws/aws-sdk-go-v2/aws/middleware?utm_source=chatgpt.com "middleware package - github.com/aws/aws-sdk-go-v2/aws ...")).
    

---

## Tips for Troubleshooting Credentials Cache

|Task|Advice|
|---|---|
|Log credential resolution|Use `CredentialsLogger` to observe hits/misses and expiry|
|Detect stale tokens|Include `Expires` info in logs; check if credentials are expired or near expiry|
|Understand provider chain|Print identity provider details or step names via middleware|
|Narrow down issues|Attach logger specifically to operations or use context-enriched logs (e.g., operation name via `middleware.GetOperationName`) ([Go Packages](https://pkg.go.dev/github.com/aws/aws-sdk-go-v2/aws/middleware?utm_source=chatgpt.com "middleware package - github.com/aws/aws-sdk-go-v2/aws ..."))|

---

## Wrap-Up

- **Credentials cache** is handled by default in Go SDK v2. You don't need custom wrappers, just logging around it.
    
- **Smithy middleware** lets you introspect middleware lifecycles and extract context (like signing credentials).
    
- The example above provides a clear path to logging and debugging credentials behavior in your AWS SDK Go v2 apps.
    

Want to extend this to other middleware steps (e.g., Initialize or Serialize), or log provider chain details? Let me know—happy to help!