# position_marks

## Version history

- v1: `position_marks`: `positionmarks.NewPositionMarksFunc(market broker.Broker, aliases []string, acct store.AccountReader, w mark.Writer, clk clock.Clock)` is the constructor of the `position_marks` job (`internal/jobs/positionmarks.New`, declared only until its entry lands); `market` is the one market-data gateway whatever account holds the position, `aliases` are the accounts whose positions are marked, the main one first (docs/design/position-marks.md#Interface).
