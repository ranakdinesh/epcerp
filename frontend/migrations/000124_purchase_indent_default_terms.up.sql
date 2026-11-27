ALTER TABLE public.application_purchase_settings
    ADD COLUMN indent_default_terms text NOT NULL DEFAULT '',
    ADD COLUMN indent_terms_version integer NOT NULL DEFAULT 0,
    ADD CONSTRAINT application_purchase_settings_indent_terms_length_check
        CHECK (char_length(indent_default_terms) <= 12000);
