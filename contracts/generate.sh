#!/bin/sh
# Regenerate everything derived from contracts/: Go records, dataset pages, TS stub.
# Go tools only (go.mod tool directive and contracts/gendoc); no npx, no download beyond the module cache.
set -eu
cd "$(dirname "$0")/.."

D=https://optdesk.invalid/datasets
OUT=internal/contracts/gen/records.go
go tool go-jsonschema -p gen --capitalization ID \
  --schema-root-type "$D/master.session_day.r1.json=SessionDay" \
  --schema-root-type "$D/obs.series_point.r1.json=SeriesPoint" \
  --schema-root-type "$D/obs.option_eod.r1.json=OptionEod" \
  --schema-root-type "$D/obs.quote_snapshot.r1.json=QuoteSnapshot" \
  --schema-root-type "$D/readings.reading.r1.json=ReadingRecord" \
  --schema-root-type "$D/obs.fundamental_fact.r1.json=FundamentalFact" \
  --schema-root-type "$D/candidates.option_candidate.r1.json=OptionCandidate" \
  --schema-root-type "$D/optionledger.flex_trade_row.r1.json=FlexTradeRow" \
  --schema-root-type "$D/optionledger.flex_fetch.r2.json=FlexFetch" \
  --schema-root-type "$D/optionledger.flex_section_row.r1.json=FlexSectionRow" \
  --schema-root-type "$D/optionledger.flex_section_status.r1.json=FlexSectionStatus" \
  --schema-root-type "$D/obs.earnings_date.r1.json=EarningsDate" \
  --schema-root-type "$D/account.position.r1.json=AccountPosition" \
  --schema-root-type "$D/account.value.r1.json=AccountValue" \
  --schema-root-type "$D/account.execution.r1.json=AccountExecution" \
  --schema-root-type "$D/obs.price_bar.r1.json=PriceBar" \
  --schema-root-type "$D/obs.fx_rate.r1.json=FxRate" \
  --schema-root-type "$D/account.position_mark.r1.json=AccountPositionMark" \
  --schema-root-type "$D/obs.dividend_expectation.r1.json=DividendExpectation" \
  --schema-root-type "$D/obs.option_open.r1.json=OptionOpen" \
  --schema-root-type "$D/plan.option_open_item.r1.json=OptionOpenItem" \
  --schema-root-type "$D/master.option_not_listed.r1.json=OptionNotListed" \
  --schema-root-type "$D/strategy.result.r1.json=StrategyResult" \
  --schema-output "$D/master.session_day.r1.json=$OUT" \
  --schema-output "$D/obs.series_point.r1.json=$OUT" \
  --schema-output "$D/obs.option_eod.r1.json=$OUT" \
  --schema-output "$D/obs.quote_snapshot.r1.json=$OUT" \
  --schema-output "$D/readings.reading.r1.json=$OUT" \
  --schema-output "$D/obs.fundamental_fact.r1.json=$OUT" \
  --schema-output "$D/candidates.option_candidate.r1.json=$OUT" \
  --schema-output "$D/optionledger.flex_trade_row.r1.json=$OUT" \
  --schema-output "$D/optionledger.flex_fetch.r2.json=$OUT" \
  --schema-output "$D/optionledger.flex_section_row.r1.json=$OUT" \
  --schema-output "$D/optionledger.flex_section_status.r1.json=$OUT" \
  --schema-output "$D/obs.earnings_date.r1.json=$OUT" \
  --schema-output "$D/account.position.r1.json=$OUT" \
  --schema-output "$D/account.value.r1.json=$OUT" \
  --schema-output "$D/account.execution.r1.json=$OUT" \
  --schema-output "$D/obs.price_bar.r1.json=$OUT" \
  --schema-output "$D/obs.fx_rate.r1.json=$OUT" \
  --schema-output "$D/account.position_mark.r1.json=$OUT" \
  --schema-output "$D/obs.dividend_expectation.r1.json=$OUT" \
  --schema-output "$D/obs.option_open.r1.json=$OUT" \
  --schema-output "$D/plan.option_open_item.r1.json=$OUT" \
  --schema-output "$D/master.option_not_listed.r1.json=$OUT" \
  --schema-output "$D/strategy.result.r1.json=$OUT" \
  -o "$OUT" \
  contracts/datasets/master.session_day.r1.json contracts/datasets/obs.series_point.r1.json contracts/datasets/obs.option_eod.r1.json contracts/datasets/obs.quote_snapshot.r1.json contracts/datasets/readings.reading.r1.json contracts/datasets/obs.fundamental_fact.r1.json contracts/datasets/candidates.option_candidate.r1.json contracts/datasets/optionledger.flex_trade_row.r1.json contracts/datasets/optionledger.flex_fetch.r2.json contracts/datasets/optionledger.flex_section_row.r1.json contracts/datasets/optionledger.flex_section_status.r1.json contracts/datasets/obs.earnings_date.r1.json contracts/datasets/account.position.r1.json contracts/datasets/account.value.r1.json contracts/datasets/account.execution.r1.json contracts/datasets/obs.price_bar.r1.json contracts/datasets/obs.fx_rate.r1.json contracts/datasets/account.position_mark.r1.json contracts/datasets/obs.dividend_expectation.r1.json contracts/datasets/obs.option_open.r1.json contracts/datasets/plan.option_open_item.r1.json contracts/datasets/master.option_not_listed.r1.json contracts/datasets/strategy.result.r1.json
gofmt -w internal/contracts/gen/records.go

go run ./contracts/gendoc -in contracts/datasets -out docs/contracts/datasets

if [ -f contracts/generate-proto.sh ]; then
  sh contracts/generate-proto.sh
fi
