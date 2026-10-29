-- Which prefilled fields the payer may change in Swish. The defaults keep
-- what every link did before: only the message is editable.
ALTER TABLE swosh
    ADD COLUMN payee_editable   boolean NOT NULL DEFAULT false,
    ADD COLUMN amount_editable  boolean NOT NULL DEFAULT false,
    ADD COLUMN message_editable boolean NOT NULL DEFAULT true;
