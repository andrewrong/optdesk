# Protocol: versions of the contracts

Every contract carries a version in its name: a record is rN (`master.session_day.r1`), a page schema is vN (`page.macro_rates` v1), the proto package is `ibkr.v1` (proto v1), and a job manifest carries a manifest version. A schema version freezes at its first consumer; before that it is amended in place and the pull request's `Contracts:` line says `amended before first consumer`; after that any incompatible or additive change bumps the version. A version that has a consumer changes by a bump in its own issue.

Worked case: until pyside, the first consumer of `session.proto`, lands, the file is amended in place and the `Contracts:` line of that pull request says the version was amended before its first consumer. After that, an added rpc bumps `ibkr.v1` to `ibkr.v2`.

An additive change to a version that has consumers may instead carry `<id> vN additive` in the `Contracts:` line (`ibkr.v1.account v1 additive`); additive means a new rpc, message, field number or enum value and no removal, change or renumbering, checked by `git diff <base>...HEAD -- contracts/proto | grep -c '^-[^-]'` = 0 in the issue that uses it.
