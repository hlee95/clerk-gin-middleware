package main

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/clerkinc/clerk-sdk-go/clerk"
	"github.com/gin-gonic/gin"
	adapter "github.com/gwatts/gin-adapter"
)

// Set some leeway for checking the tokens so we don't need to keep regenerating
// them for testing purposes.
const leeway = 50000 * time.Minute

func main() {
	// Initialize clerk client.
	developmentSecretKey := os.Getenv("CLERK_DEVELOPMENT_SECRET_KEY")
	clerkClient, err := clerk.NewClient(developmentSecretKey)
	if err != nil {
		panic(err)
	}

	// Create two gin router groups, one with middleware and one without.
	baseRouter := gin.Default()
	routerWithMiddleware := baseRouter.Group("/")
	// routerWithMiddleware.Use(clerkMiddlewareAttempt1(clerkClient)) // uncomment to try this one.
	routerWithMiddleware.Use(clerkMiddlewareAttempt2(clerkClient))

	routerNoMiddleware := baseRouter.Group("/")

	// new middleware, using package adapter to convert the net/http middleware
	// to a Gin-compatible middleware
	routerWithMiddleware2 := baseRouter.Group("/")
	routerWithMiddleware2.Use(adapter.Wrap(clerk.WithSessionV2(clerkClient, clerk.WithLeeway(leeway))))
	routerWithMiddleware2.GET("/user2", func(c *gin.Context) {
		claims, ok := clerk.SessionFromContext(c.Request.Context())
		fmt.Println(ok)
		fmt.Println(claims)
	})

	// This endpoint is a dummy health check to prove the server is up.
	baseRouter.GET("/livez", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	// This endpoint reads and verifies the bearer token directly to parse claims
	// and fetch the user.
	routerNoMiddleware.GET("/user", func(c *gin.Context) {
		sessionToken := c.Request.Header.Get("Authorization")
		sessionToken = strings.TrimPrefix(sessionToken, "Bearer ")
		sessClaims, err := clerkClient.VerifyToken(sessionToken, clerk.WithLeeway(leeway))
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{
				"message": "Unauthorized",
			})
			return
		}
		user, err := clerkClient.Users().Read(sessClaims.Claims.Subject)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"message": "failed to read user from claims",
			})
		}
		c.JSON(http.StatusOK, gin.H{
			"firstName": user.FirstName,
			"lastName":  user.LastName,
		})
	})

	// This endpoint relies on the session claims being available in the
	// context of the request, and then uses the session claims to fetch the user.
	// I expect the session to be available, but it is not.
	routerWithMiddleware.GET("/user-with-middleware", func(c *gin.Context) {
		clerkSessionClaims, found := clerk.SessionFromContext(c.Request.Context())
		if !found {
			c.JSON(http.StatusInternalServerError, gin.H{
				"message": "failed to retrieve session from context",
			})
		}
		user, err := clerkClient.Users().Read(clerkSessionClaims.Claims.Subject)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"message": "failed to read user from claims",
			})
		}
		c.JSON(http.StatusOK, gin.H{
			"firstName": user.FirstName,
			"lastName":  user.LastName,
		})
	})

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", 4242),
		Handler: baseRouter,
	}

	// Skipping graceful shutdown logic since this is just an example.
	srv.ListenAndServe()
}

func clerkMiddlewareAttempt1(clerkClient clerk.Client) gin.HandlerFunc {
	checkAndInjectActiveSession := clerk.RequireSessionV2(clerkClient, clerk.WithLeeway(leeway))

	var noop http.HandlerFunc = func(w http.ResponseWriter, r *http.Request) {
		_, found := clerk.SessionFromContext(r.Context())
		if found {
			fmt.Println("found session in noop handler") // <-- this logs, so the session is passed down into here. but, it's not available in the gin handler.
		} else {
			fmt.Println("did not find session in noop handler")
		}
	}

	return gin.WrapH(checkAndInjectActiveSession(noop))
}

// This doesn't use gin.H and instead tries to wrap the Clerk middleware manually.
func clerkMiddlewareAttempt2(clerkClient clerk.Client) gin.HandlerFunc {
	checkAndInjectActiveSession := clerk.RequireSessionV2(clerkClient, clerk.WithLeeway(leeway))

	var noop http.HandlerFunc = func(w http.ResponseWriter, r *http.Request) {
		_, found := clerk.SessionFromContext(r.Context())
		if found {
			fmt.Println("found session in noop handler") // <-- this logs, so the session is passed down into here. but, it's not available in the gin handler.
		} else {
			fmt.Println("did not find session in noop handler")
		}
	}

	return func(c *gin.Context) {
		handler := checkAndInjectActiveSession(noop)

		handler.ServeHTTP(c.Writer, c.Request) // <- runs the handler provided by clerk middleware

		_, found := clerk.SessionFromContext(c.Request.Context())
		// If you run this, the session is NOT found, because the Clerk middleware doesn't modify the request.
		// So, downstream gin handlers will not have accesss to the session in the request context.
		if found {
			fmt.Println("found session in middleware")
		} else {
			fmt.Println("did not find session in middleware") // <-- this logs
		}
		// If the status has been written by the middleware, return out.
		if c.Writer.Status() != http.StatusOK {
			c.Abort()
			return
		}
		c.Next()
	}
}
