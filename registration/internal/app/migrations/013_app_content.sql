CREATE TABLE app_content (
 id text PRIMARY KEY,
 document jsonb NOT NULL CHECK(jsonb_typeof(document) = 'object'),
 published boolean NOT NULL DEFAULT true,
 updated_at timestamptz NOT NULL DEFAULT now()
);

INSERT INTO app_content(id, document) VALUES ('mobile', $content$
{
  "event": {
    "title": "Bio Connect 4.0",
    "tagline": "Connecting Science to Business",
    "hero_title": "Where science\nmeets what’s next.",
    "description": "Kerala's international life sciences conclave and expo. A platform for science, enterprise and policy to meet.",
    "start_date": "2026-10-08",
    "end_date": "2026-10-09",
    "venue": "Hyatt Regency Trivandrum",
    "city": "Thiruvananthapuram, Kerala",
    "brochure_url": "https://bioconnect.kerala.gov.in/assets/bio-connect-4-brochure.pdf",
    "sponsorship_email": "bioconnect@bio360.in"
  },
  "themes": [
    {"title":"Biopharma","description":"From discovery to next-generation healthcare.","image":"assets/images/theme-biopharma.webp"},
    {"title":"MedTech","description":"Engineering the future of healthcare.","image":"assets/images/theme-medtech.webp"},
    {"title":"Nutraceuticals","description":"Harnessing nature for health and wellness.","image":"assets/images/theme-nutraceuticals.webp"},
    {"title":"Agri-Food Innovation","description":"Innovations for food security and sustainability.","image":"assets/images/theme-agrifood.webp"},
    {"title":"Artificial Intelligence","description":"Transforming life sciences through AI and digital innovation.","image":"assets/images/theme-ai.webp"}
  ],
  "programme_highlights": [
    "Technology expo", "Product launch", "R&D resource connect", "B2B & B2G meetups",
    "Panel discussions", "Investor connect", "Industry & startup showcase", "Networking"
  ],
  "product_launch": {
    "eyebrow": "Kerala Startup Mission",
    "title": "Bring your product into the spotlight.",
    "description": "A launch stage for practical life-sciences solutions across Bio Connect's five focus areas.",
    "deadline": "30 September 2026",
    "eligibility": [
      {"title":"Life sciences startups","description":"Startups with DPIIT registration."},
      {"title":"Small and medium enterprises","description":"SMEs preparing a life-sciences product for launch."}
    ],
    "focus_areas": ["Biopharma", "MedTech", "Nutraceuticals", "Agri-Food Innovation", "Artificial Intelligence"],
    "apply_url": "https://ksum.in/BioConnect"
  },
  "leadership": {
    "intro": "Meet the leadership and organisations building Kerala's life sciences ecosystem.",
    "advisory_note": "A 14-member committee brings government, research and industry together to steer the conclave's strategy and delivery.",
    "people": [
      {"name":"V. D. Satheesan","role":"Hon'ble Chief Minister, Government of Kerala","badge":"Chief Minister"},
      {"name":"P. K. Kunhalikutty","role":"Hon'ble Minister for Industries and Information Technology, Government of Kerala","badge":"Chairman, Advisory Committee"}
    ]
  },
  "sponsor": {
    "name": "Kerala Rubber Limited",
    "description": "A Government of Kerala initiative strengthening natural-rubber manufacturing, MSMEs and connections between growers, industry and investors.",
    "logo_url": "https://bioconnect.kerala.gov.in/assets/kerala-rubber-logo.png",
    "website_url": "https://keralarubber.com/"
  },
  "ecosystem_partners": [
    {"name":"LSDC","logo_url":"https://bioconnect.kerala.gov.in/assets/lsdc-logo.png"},
    {"name":"Kerala Startup Mission","logo_url":"https://bioconnect.kerala.gov.in/assets/ksum-logo.png"},
    {"name":"Kerala State Council for Science, Technology and Environment","logo_url":"https://bioconnect.kerala.gov.in/assets/kscste-logo.png"},
    {"name":"Kerala Biotechnology Board","logo_url":"https://bioconnect.kerala.gov.in/assets/kbb-logo.png"}
  ]
}
$content$::jsonb);
