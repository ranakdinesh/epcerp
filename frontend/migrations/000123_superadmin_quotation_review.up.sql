BEGIN;

CREATE FUNCTION public.check_quotation_reviewer_separation() RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    IF NEW.created_by_id = NEW.submitted_by_id AND (
        current_setting('epcerp.auth_actor', true) IS DISTINCT FROM NEW.created_by_id::text
        OR NOT EXISTS (
            SELECT 1 FROM auth.users
            WHERE id = NEW.created_by_id AND is_active AND is_superuser AND NOT must_change_password
        )
    ) THEN
        RAISE EXCEPTION 'Only the authenticated Superadmin can review their own quotation'
            USING ERRCODE = '23514';
    END IF;
    RETURN NEW;
END;
$$;

ALTER TABLE public.finance_quotation_decisions
    DROP CONSTRAINT finance_quotation_decisions_check;

CREATE TRIGGER quotation_reviewer_separation
    BEFORE INSERT ON public.finance_quotation_decisions
    FOR EACH ROW EXECUTE FUNCTION public.check_quotation_reviewer_separation();

COMMIT;
