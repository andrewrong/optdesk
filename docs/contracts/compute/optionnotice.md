# compute/optionnotice

Each `CashByDate` row (assignment cash due on a day in one currency) also carries `Available` and `Left`. When the account's `TotalCashValue` row has the same currency as the cash row, `Available` is that value and `Left` is `Available` minus the `Amount` of this row and of the earlier rows of the same currency (a negative `Left` is a shortfall, written with its sign); a row of another currency, and every row when there is no `TotalCashValue` row, has both empty. No currency is converted.
