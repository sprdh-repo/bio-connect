ALTER TABLE categories
 ADD COLUMN free_only boolean NOT NULL DEFAULT false;

INSERT INTO categories(id,kind,label,early_paise,regular_paise,roster_count,open,free_only)
 VALUES('official','delegate','Govt. Official',1,1,1,true,true);
