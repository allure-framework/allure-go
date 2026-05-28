# allure-go testify

`github.com/allure-framework/allure-go/testify` contains drop-in proxies for Testify `assert` and `require` packages.

Use the proxy imports when you want Testify assertion calls to appear as Allure steps:

```diff
 import (
-	"github.com/stretchr/testify/assert"
-	"github.com/stretchr/testify/require"
+	"github.com/allure-framework/allure-go/testify/assert"
+	"github.com/allure-framework/allure-go/testify/require"
 )
```

Replacing only the imports keeps normal Testify behavior for assertions that still receive `t *testing.T`:

```go
assert.Equal(t, expected, actual)
require.NoError(t, err)
```

Pass an Allure-aware test context when you want assertion calls to be reported as Allure steps. The context must satisfy Testify's testing interface and `commons.ContextProvider`; `gotest.Context` already does both:

```go
import allure "github.com/allure-framework/allure-go/commons/gotest"

allure.Test(t, "loads profile", func(a *allure.Context) {
	assert.Equal(a, expected, actual)
	require.NoError(a, err)
	assert.New(a).Len(items, 2)
})
```

Other framework integrations can expose the same behavior by implementing `Context() context.Context` on their own test context types. Passing `t` or `a.T()` keeps regular Testify behavior without assertion-step reporting. Failed context-backed assertions preserve normal Testify behavior while adding Allure status details, including expected and actual values on failed steps only.
