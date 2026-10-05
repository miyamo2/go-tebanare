package server

import "net/http"

func (l *Logger) Printf(format string, arg any) { l.printf(format, arg) }

func (t *Tracer) Printf(format string, args ...any) { t.printf(format, args...) }

func (r *Recorder) Printf(format string, args []any) { r.printf(format, args...) }

func (h *apiHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) { h.serve(w, r) }

func (h adminHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) { h.serve(w, r) }
