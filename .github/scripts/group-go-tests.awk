# Folds `go test -v` output into collapsible GitHub Actions log groups, one
# per top-level test (column-0 "=== RUN   TestX"). Subtest output stays inside
# its parent's group. Package result lines (ok/FAIL/PASS) close the current
# group so the trailing summary is visible without expanding anything.
#
# Usage: go test -v ./... 2>&1 | awk -f group-go-tests.awk
# (Groups don't nest — a new top-level test closes the previous group.)

/^(ok[ \t]|FAIL|PASS$)/ {
	if (open) { print "::endgroup::"; open = 0 }
}

/^=== RUN   Test/ && $3 !~ /\// {
	if (open) print "::endgroup::"
	print "::group::" $3
	open = 1
}

{ print }

END {
	if (open) print "::endgroup::"
}
