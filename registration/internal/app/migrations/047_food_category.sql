-- Food passes cover meals only: drivers and other support staff who come with
-- delegates. Staff issue them from the console or the spot desk and send them
-- like any pass; the pass, the badge and the email all say FOOD ONLY (foodOnly).
-- Staff-only: closed to paid registration and to free links.
INSERT INTO categories(id,kind,label,early_paise,regular_paise,roster_count,open,free_only,free_open)
 VALUES('food','delegate','Food Pass',1,1,1,false,true,false);
