// Package analyzer runs a compiled rule set against Go source files and
// computes the lines to hide.
//
// AnalyzeFile handles one version of a file: it checks the size and
// nesting limits, parses the file, finds the declarations, statements, and
// expressions that the rules match, and keeps a match only when its lines
// hold no other code. AnalyzeChange analyzes both sides of a change and
// pairs the func rule matches of the two sides by declaration key, so a
// function whose match state changed stays visible.
//
// Every line number is a line of the file itself: //line directives are
// ignored.
package analyzer
