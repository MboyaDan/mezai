-- Stage 17a: the name M-Pesa returns with each B2C payment is compared with
-- the contributor's name when the callback arrives (D-139). Only the result
-- is kept; the name (and the phone number beside it) is removed from the
-- stored callback body. A mismatch is recorded on the profile, which holds
-- further payouts until Ops has checked.

-- +goose Up
ALTER TABLE mpesa_callbacks ADD COLUMN name_check text CHECK (name_check IN ('match', 'mismatch', 'unavailable'));
ALTER TABLE mpesa_transactions ADD COLUMN name_check text CHECK (name_check IN ('match', 'mismatch', 'unavailable'));
ALTER TABLE contributor_profiles ADD COLUMN mpesa_name_mismatch_at timestamptz;

-- Callbacks stored by Stage 15a (development and test only: B2C has never
-- run for a real person) still hold the name parameter. Remove it.
UPDATE mpesa_callbacks SET body = jsonb_set(body, '{Result,ResultParameters,ResultParameter}',
  (SELECT coalesce(jsonb_agg(CASE WHEN p->>'Key' IN ('ReceiverPartyPublicName', 'CreditPartyName', 'DebitPartyName')
                             THEN jsonb_set(p, '{Value}', '"[removed]"') ELSE p END), '[]'::jsonb)
   FROM jsonb_array_elements(body #> '{Result,ResultParameters,ResultParameter}') p))
WHERE jsonb_typeof(body #> '{Result,ResultParameters,ResultParameter}') = 'array';

-- +goose Down
ALTER TABLE contributor_profiles DROP COLUMN mpesa_name_mismatch_at;
ALTER TABLE mpesa_transactions DROP COLUMN name_check;
ALTER TABLE mpesa_callbacks DROP COLUMN name_check;
