package gist

import (
	"regexp"

	"gopkg.in/kataras/iris.v6"
)

// Expr takes pairs with the named path (without symbols) following by its expression
// and returns a middleware which will do a pure but effective validation using the regexp package.
func Expr(pairParamExpr ...string) iris.HandlerFunc {
	srvErr := func(ctx *iris.Context) {
		ctx.EmitError(iris.StatusInternalServerError)
	}

	if len(pairParamExpr)%2 != 0 {
		App.Log(iris.ProdMode,
			"regexp pre-compile error: the format is paramName, expression"+
				"paramName2, expression2. The len should be %2==0")
		return srvErr
	}
	pairs := make(map[string]*regexp.Regexp, len(pairParamExpr)/2)

	for i := 0; i < len(pairParamExpr)-1; i++ {
		expr := pairParamExpr[i+1]
		r, err := regexp.Compile(expr)
		if err != nil {
			App.Log(iris.ProdMode, "regexp failed on: "+expr+". Trace:"+err.Error())
			return srvErr
		}

		pairs[pairParamExpr[i]] = r
		i++
	}

	// return the middleware
	return func(ctx *iris.Context) {
		for k, v := range pairs {
			pathPart := ctx.Param(k)
			if pathPart == "" {
				// take care, the router already
				// does the param validations
				// so if it's empty here it means that
				// the router has label it as optional.
				// so we skip it, and continue to the next.
				continue
			}
			// the improtant thing:
			// if the path part didn't match with the relative exp, then fire status not found.
			if !v.MatchString(pathPart) {
				ctx.EmitError(iris.StatusNotFound)
				return
			}
		}
		// otherwise continue to the next handler...
		ctx.Next()
	}
}
