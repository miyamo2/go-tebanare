// Package astmatch compares go/ast type expressions structurally.
//
// A pattern is an ordinary go/ast expression that may contain sentinel
// identifiers: Any matches one type, Seq matches zero or more list elements,
// and TParam refers to a pattern type parameter that an Env binds to a
// source type parameter.
//
// The package resolves nothing. It never reads imports, declarations, or
// other files, so two types match only when they are spelled the same way,
// apart from the equivalences listed on MatchType.
package astmatch
