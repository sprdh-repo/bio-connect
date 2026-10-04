-- Six speakers from the organisers' folder on 2026-10-04, appended after the 017 line-up.
-- Only Dr. Piyush Das came with a portrait; the others keep an empty image_slug and show monograms.
INSERT INTO speakers(position,id,name,role,organization,image_slug,linkedin) VALUES
 (51,'balakrishnan-t-p','Balakrishnan T. P.','Founder & CEO','I2R Labs','','https://www.linkedin.com/in/balakrishnan-tp-34565b1b/'),
 (52,'binu-augustin','Binu Augustin','Director','Heka Medicals India Pvt. Ltd.','','https://www.linkedin.com/in/binu-augustin-31034424/'),
 (53,'c-padmakumar','C. Padmakumar','Special Officer','Kerala Medical Technology Consortium (KMTC)','','https://www.linkedin.com/in/padmakumar-advisory/'),
 (54,'joseph-jose','Joseph Jose','Managing Director','Mariya Healthcare Pvt. Ltd.','','https://www.linkedin.com/in/joseph-jose-12063696/'),
 (55,'t-c-jayasankar','T. C. Jayasankar','Managing Director','Jayon Implants Pvt. Ltd.','',''),
 (56,'piyush-das','Dr. Piyush Das','Country Director & General Manager, India and South Asia','Haier Biomedical','piyush-das','');

-- Dr. Sreeja Narayanan sent a portrait and her current title.
UPDATE speakers SET role='Faculty and Principal Investigator, Nano-Bioengineering', image_slug='sreeja-narayanan', updated_at=now() WHERE id='sreeja-narayanan';
