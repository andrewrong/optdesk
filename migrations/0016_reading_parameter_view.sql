-- +goose Up
-- rd.reading_parameter_r1: the parameters of readings.reading_parameter, so the page and notify roles read the
-- parameter data_day without a grant on the base table (docs/design/report-page-producer.md Decision 7).
CREATE VIEW rd.reading_parameter_r1 AS
SELECT name, version, subject_kind, subject, day, rev, param, value_num, value_tag
FROM readings.reading_parameter;

ALTER VIEW rd.reading_parameter_r1 OWNER TO ot_owner;

GRANT SELECT ON rd.reading_parameter_r1 TO ot_ro_pages, ot_w_readings;

-- +goose Down
REVOKE SELECT ON rd.reading_parameter_r1 FROM ot_ro_pages, ot_w_readings;
DROP VIEW rd.reading_parameter_r1;
