-- Portraits for the KMTC/KMDIA session speakers arrived on 2026-10-05, with an updated profile for Binu Augustin.
UPDATE speakers SET image_slug=id, updated_at=now()
 WHERE id IN ('balakrishnan-t-p','binu-augustin','c-padmakumar','joseph-jose','t-c-jayasankar');
UPDATE speakers SET role='Managing Director & Co-founder', updated_at=now() WHERE id='binu-augustin';
