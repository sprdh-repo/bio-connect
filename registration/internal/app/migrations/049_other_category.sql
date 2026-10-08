-- Other is the catch-all for anyone no other category fits. Staff issue it
-- from the console or the spot desk with only a name; organisation,
-- designation, email and phone are optional (anyDetails). The pass is sent to
-- any email or phone given, and can always be downloaded and handed over.
-- Staff-only: closed to paid registration and to free links.
INSERT INTO categories(id,kind,label,early_paise,regular_paise,roster_count,open,free_only,free_open)
 VALUES('other','delegate','Other',1,1,1,false,true,false);
