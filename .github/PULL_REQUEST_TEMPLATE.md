## Summary

<!-- What and why -->

## Testing

- [ ] `make test`
- [ ] `make build` (and `make build-noneighbor` if neighbor-related)
- [ ] Manual check on Linux host (describe) — or N/A

## Checklist

- [ ] Dependencies updated with `make vendor` if `go.mod` changed
- [ ] Docs updated if flags / behavior changed
- [ ] Dashboards / rules updated and `make check-assets` passes if metrics changed
- [ ] No new high-cardinality labels without caps
- [ ] Security / privilege impact noted (if any)
