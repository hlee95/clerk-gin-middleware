# clerk-gin-middleware

This is an example trying to use the provided Clerk middleware, specifically `RequireSessionV2`, as a gin middleware function (`gin.HandlerFunc`).

Run the example:

- `go build ./...`
- `CLERK_DEVELOPMENT_SECRET_KEY=sk_test_redacted go run main.go` Use your own clerk key.

There are three endpoints.

- `/livez` is a health check
- `/user` fetches the user by directly parsing the token from the headers, without using any middleware
- `/user-with-middleware` is wrapped in middleware and tries to read the session out of the request context

This is the example that I use.

```
curl --location --request GET 'http://localhost:4242/user-with-middleware' \
--header 'Authorization: Bearer <token>'
```

I set the leeway very high so the token I'm passing continues to work and I don't need to refresh it.

For `http://localhost:4242/user`, it all works find and it's able to retrieve the user:

```
[GIN] 2023/02/03 - 11:32:54 | 200 |  222.940625ms |       127.0.0.1 | GET      "/user"
```

and it returns:

```
{"firstName":"Hanna","lastName":"Lee"}
```

However, for `http://localhost:4242/user-with-middleware`, it logs this:

```
found session in noop handler
did not find session in middleware


2023/02/03 11:27:46 [Recovery] 2023/02/03 - 11:27:46 panic recovered:
GET /user-with-middleware HTTP/1.1
Host: localhost:4242
Accept: */*
Authorization: *
User-Agent: curl/7.84.0


runtime error: invalid memory address or nil pointer dereference
/opt/homebrew/Cellar/go/1.19.5/libexec/src/runtime/panic.go:260 (0x1046034b7)
	panicmem: panic(memoryError)
/opt/homebrew/Cellar/go/1.19.5/libexec/src/runtime/signal_unix.go:835 (0x10461b227)
	sigpanic: panicmem()
/Users/hannalee/sandbox/clerk-gin-middleware/main.go:73 (0x1048f9320)
	main.func3: user, err := clerkClient.Users().Read(clerkSessionClaims.Claims.Subject)
/Users/hannalee/go/pkg/mod/github.com/gin-gonic/gin@v1.8.2/context.go:173 (0x1048f9d0b)
	(*Context).Next: c.handlers[c.index](c)
/Users/hannalee/sandbox/clerk-gin-middleware/main.go:139 (0x1048f9ccc)
	clerkMiddlewareAttempt2.func2: c.Next()
/Users/hannalee/go/pkg/mod/github.com/gin-gonic/gin@v1.8.2/context.go:173 (0x1048f3c6b)
	(*Context).Next: c.handlers[c.index](c)
/Users/hannalee/go/pkg/mod/github.com/gin-gonic/gin@v1.8.2/recovery.go:101 (0x1048f3c4c)
	CustomRecoveryWithWriter.func1: c.Next()
/Users/hannalee/go/pkg/mod/github.com/gin-gonic/gin@v1.8.2/context.go:173 (0x1048f2eeb)
	(*Context).Next: c.handlers[c.index](c)
/Users/hannalee/go/pkg/mod/github.com/gin-gonic/gin@v1.8.2/logger.go:240 (0x1048f2ec8)
	LoggerWithConfig.func1: c.Next()
/Users/hannalee/go/pkg/mod/github.com/gin-gonic/gin@v1.8.2/context.go:173 (0x1048f2073)
	(*Context).Next: c.handlers[c.index](c)
/Users/hannalee/go/pkg/mod/github.com/gin-gonic/gin@v1.8.2/gin.go:616 (0x1048f1d58)
	(*Engine).handleHTTPRequest: c.Next()
/Users/hannalee/go/pkg/mod/github.com/gin-gonic/gin@v1.8.2/gin.go:572 (0x1048f1ab3)
	(*Engine).ServeHTTP: engine.handleHTTPRequest(c)
/opt/homebrew/Cellar/go/1.19.5/libexec/src/net/http/server.go:2947 (0x1047cf813)
	serverHandler.ServeHTTP: handler.ServeHTTP(rw, req)
/opt/homebrew/Cellar/go/1.19.5/libexec/src/net/http/server.go:1991 (0x1047cc74f)
	(*conn).serve: serverHandler{c.server}.ServeHTTP(w, w.req)
/opt/homebrew/Cellar/go/1.19.5/libexec/src/runtime/asm_arm64.s:1172 (0x104634ef3)
	goexit: MOVD	R0, R0	// NOP
```

Essentially, I believe the issue is that if the Clerk middleware adds to the request context, it should modify the request context. Instead, it is creating a new one (https://github.com/clerkinc/clerk-sdk-go/blob/main/clerk/middleware_v2.go#L135).

Instead of:

```
				ctx := context.WithValue(r.Context(), ActiveSessionClaims, claims)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
```

perhaps we could do:

```
				ctx := context.WithValue(r.Context(), ActiveSessionClaims, claims)
        r = r.WithContext(ctx)
				next.ServeHTTP(w, r)
				return
```
