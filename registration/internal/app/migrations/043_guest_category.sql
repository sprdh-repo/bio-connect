-- Guests get a complimentary one-person pass that staff issue from the console
-- or the spot desk. Only the name is required, and the pass is never sent:
-- staff download it and hand it over (see downloadOnly). It is staff-only:
-- closed to paid registration and to free links, so no public form offers it.
INSERT INTO categories(id,kind,label,early_paise,regular_paise,roster_count,open,free_only,free_open)
 VALUES('guest','delegate','Guest',1,1,1,false,true,false);
