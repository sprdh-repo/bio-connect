-- Organisers, sponsors and volunteers register free, like government officials: one pass
-- each, through a free registration link or staff entry only.
INSERT INTO categories(id,kind,label,early_paise,regular_paise,roster_count,open,free_only)
 VALUES('organiser','delegate','Organiser',1,1,1,true,true),
       ('sponsor','delegate','Sponsor',1,1,1,true,true),
       ('volunteer','delegate','Volunteer',1,1,1,true,true);

-- Official passes were rendered with the exhibitor treatment. Drop their
-- cached PDFs so each is rendered again as OFFICIAL on its next download;
-- numbers and QR codes are unchanged.
UPDATE passes SET file_id=NULL
 WHERE revoked_at IS NULL AND registration_id IN (SELECT id FROM registrations WHERE category_id='official');
