// Package statementcapture is the statement_capture job: for each statement-sourced alias it turns the stored
// Flex sections OpenPosition, EquitySummaryByReportDateInBase and ChangeInNAV into one account capture per
// report day and one mark per position (Decisions 2 to 6, 9 and 10 of docs/design/statement-account.md).
package statementcapture
