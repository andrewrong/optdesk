-- +goose Up
-- ops.rename_account_alias(old, new): renames the alias of the rows of one account in the nine tables that carry one
-- (docs/design/account-identity.md, entry 3). It renames nothing by itself: the window calls it once per old alias,
-- as the owner role (ot_owner); the writer roles hold no UPDATE and keep none. optionledger.flex_fetch and
-- ops.capture_expected_key have no alias column and are not touched. No table, column or view changes here.
-- The whole call is one transaction (a function runs inside the caller's). Before the first UPDATE, a row under the
-- new alias with the key of a row under the old alias (the key with alias replaced) in any table raises and
-- nothing changes. Where alias is not part of the primary key (account.capture, account.position, account.value,
-- account.position_mark) the key cannot collide and the check finds nothing.
-- +goose StatementBegin
CREATE FUNCTION ops.rename_account_alias(old_alias text, new_alias text) RETURNS void
LANGUAGE plpgsql AS $$
BEGIN
    IF old_alias = new_alias THEN
        RAISE EXCEPTION 'rename_account_alias: old and new alias are the same: %', old_alias;
    END IF;
    IF EXISTS (SELECT 1 FROM account.capture n JOIN account.capture o ON o.capture_id = n.capture_id
               WHERE n.alias = new_alias AND o.alias = old_alias) THEN
        RAISE EXCEPTION 'rename_account_alias: account.capture already has a row under alias % with the key of a row under alias %', new_alias, old_alias;
    END IF;
    IF EXISTS (SELECT 1 FROM account.position n JOIN account.position o ON o.capture_id = n.capture_id AND o.con_id = n.con_id
               WHERE n.alias = new_alias AND o.alias = old_alias) THEN
        RAISE EXCEPTION 'rename_account_alias: account.position already has a row under alias % with the key of a row under alias %', new_alias, old_alias;
    END IF;
    IF EXISTS (SELECT 1 FROM account.value n JOIN account.value o ON o.capture_id = n.capture_id AND o.tag = n.tag AND o.currency = n.currency
               WHERE n.alias = new_alias AND o.alias = old_alias) THEN
        RAISE EXCEPTION 'rename_account_alias: account.value already has a row under alias % with the key of a row under alias %', new_alias, old_alias;
    END IF;
    IF EXISTS (SELECT 1 FROM account.execution n JOIN account.execution o ON o.exec_id = n.exec_id
               WHERE n.alias = new_alias AND o.alias = old_alias) THEN
        RAISE EXCEPTION 'rename_account_alias: account.execution already has a row under alias % with the key of a row under alias %', new_alias, old_alias;
    END IF;
    IF EXISTS (SELECT 1 FROM account.position_mark n JOIN account.position_mark o ON o.capture_id = n.capture_id AND o.con_id = n.con_id AND o.attempt = n.attempt
               WHERE n.alias = new_alias AND o.alias = old_alias) THEN
        RAISE EXCEPTION 'rename_account_alias: account.position_mark already has a row under alias % with the key of a row under alias %', new_alias, old_alias;
    END IF;
    IF EXISTS (SELECT 1 FROM candidates.option_candidate n JOIN candidates.option_candidate o ON o.session_day = n.session_day AND o.run_id = n.run_id AND o.kind = n.kind AND o.instrument_id = n.instrument_id
               WHERE n.alias = new_alias AND o.alias = old_alias) THEN
        RAISE EXCEPTION 'rename_account_alias: candidates.option_candidate already has a row under alias % with the key of a row under alias %', new_alias, old_alias;
    END IF;
    IF EXISTS (SELECT 1 FROM optionledger.flex_trade_row n JOIN optionledger.flex_trade_row o ON o.level_of_detail = n.level_of_detail AND o.transaction_id = n.transaction_id AND o.revision = n.revision
               WHERE n.alias = new_alias AND o.alias = old_alias) THEN
        RAISE EXCEPTION 'rename_account_alias: optionledger.flex_trade_row already has a row under alias % with the key of a row under alias %', new_alias, old_alias;
    END IF;
    IF EXISTS (SELECT 1 FROM optionledger.flex_section_row n JOIN optionledger.flex_section_row o ON o.section = n.section AND o.row_key = n.row_key AND o.revision = n.revision
               WHERE n.alias = new_alias AND o.alias = old_alias) THEN
        RAISE EXCEPTION 'rename_account_alias: optionledger.flex_section_row already has a row under alias % with the key of a row under alias %', new_alias, old_alias;
    END IF;
    IF EXISTS (SELECT 1 FROM optionledger.flex_section_status n JOIN optionledger.flex_section_status o ON o.fetch_id = n.fetch_id AND o.section = n.section
               WHERE n.alias = new_alias AND o.alias = old_alias) THEN
        RAISE EXCEPTION 'rename_account_alias: optionledger.flex_section_status already has a row under alias % with the key of a row under alias %', new_alias, old_alias;
    END IF;
    UPDATE account.capture SET alias = new_alias WHERE alias = old_alias;
    UPDATE account.position SET alias = new_alias WHERE alias = old_alias;
    UPDATE account.value SET alias = new_alias WHERE alias = old_alias;
    UPDATE account.execution SET alias = new_alias WHERE alias = old_alias;
    UPDATE account.position_mark SET alias = new_alias WHERE alias = old_alias;
    UPDATE candidates.option_candidate SET alias = new_alias WHERE alias = old_alias;
    UPDATE optionledger.flex_trade_row SET alias = new_alias WHERE alias = old_alias;
    UPDATE optionledger.flex_section_row SET alias = new_alias WHERE alias = old_alias;
    UPDATE optionledger.flex_section_status SET alias = new_alias WHERE alias = old_alias;
END;
$$;
-- +goose StatementEnd

-- +goose Down
DROP FUNCTION ops.rename_account_alias(text, text);
