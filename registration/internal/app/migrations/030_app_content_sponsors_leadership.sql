-- The mobile app's Sponsors and Leadership pages read these from app content,
-- matching sponsors.html and committee.html. The single "sponsor" object stays
-- for app versions released before the sponsors list existed.
UPDATE app_content SET updated_at = now(), document = document || $content$
{
  "sponsors_intro": "Meet the sponsors supporting Bio Connect 4.0 and the connections between science, enterprise and industry.",
  "sponsors": [
    {"name":"State Bank of India","category":"Public sector bank · Mumbai","description":"India’s largest bank, with roots going back to 1806, SBI serves more than 50 crore customers through a network of over 22,000 branches across the country.","logo_url":"https://bioconnect.kerala.gov.in/assets/sbi-logo.png","website_url":"https://sbi.co.in/"},
    {"name":"Kerala Medical Technology Consortium","category":"Government of Kerala · Thiruvananthapuram","description":"A flagship Government of Kerala initiative to make the state a hub for medical devices, bringing research, academia, industry, healthcare and government together to grow medtech innovation and manufacturing.","logo_url":"https://bioconnect.kerala.gov.in/assets/kmtc-logo.png","website_url":"https://kmtc.in/"},
    {"name":"Ministry of External Affairs - States Division","category":"Government of India · New Delhi","description":"The States Division of the Ministry of External Affairs works with state governments to strengthen their engagement with partners abroad, supported by its State Facilitation and Knowledge Support Fund.","logo_url":"https://bioconnect.kerala.gov.in/assets/mea-states-division-logo.png","website_url":"https://www.mea.gov.in/"},
    {"name":"KIMSHEALTH","category":"Healthcare network · Thiruvananthapuram","description":"Founded in Thiruvananthapuram in 2002 with a 250-bed quaternary care hospital, KIMSHEALTH has grown into a multi-specialty healthcare network across South India and the Middle East.","logo_url":"https://bioconnect.kerala.gov.in/assets/kimshealth-logo.png","website_url":"https://www.kimshealth.org/"},
    {"name":"CSIR-NIIST","category":"CSIR laboratory · Thiruvananthapuram","description":"The CSIR-National Institute for Interdisciplinary Science and Technology works across agro-processing, microbial processes, chemical and materials science, and environmental technology.","logo_url":"https://bioconnect.kerala.gov.in/assets/csir-niist-logo.png","website_url":"https://www.niist.res.in/"},
    {"name":"Federal Bank","category":"Private sector bank · Aluva, Kerala","description":"Founded in 1931 as Travancore Federal Bank, Federal Bank serves customers through more than 1,500 branches across India and handles a large share of the country’s inward remittances.","logo_url":"https://bioconnect.kerala.gov.in/assets/federal-bank-logo.png","website_url":"https://www.federal.bank.in/"},
    {"name":"Haier Biomedical & Helixpro","category":"Life science equipment · Qingdao, China","description":"Haier Biomedical makes cold-chain, laboratory and biosafety equipment covering −196°C to +8°C for storing samples, vaccines and medicines, and is a long-term supplier to WHO and UNICEF. It joins Bio Connect with its partner Helixpro.","logo_url":"https://bioconnect.kerala.gov.in/assets/haier-helixpro-logo.png","website_url":"https://www.haiermedical.com/"},
    {"name":"HLL Lifecare Limited","category":"Government of India enterprise · Thiruvananthapuram","description":"A Mini Ratna enterprise under the Ministry of Health and Family Welfare since 1966, HLL makes and delivers healthcare products and services, from contraceptives and pharmaceuticals to diagnostics.","logo_url":"https://bioconnect.kerala.gov.in/assets/hll-lifecare-logo.png","website_url":"https://www.lifecarehll.com/"},
    {"name":"Kerala Rubber Limited","category":"Government of Kerala · Kottayam","description":"Kerala Rubber Limited strengthens the state’s natural rubber industry through value-added manufacturing, supporting MSMEs and linking growers, entrepreneurs and investors.","logo_url":"https://bioconnect.kerala.gov.in/assets/kerala-rubber-logo.png","website_url":"https://keralarubber.com/"}
  ],
  "leadership": {
    "intro": "Bio Connect 4.0 is convened under a Government Order that also constituted the Advisory Committee guiding it - the State's government, research and industry leadership at one table.",
    "advisory_note": "The Advisory Committee steers the conclave - from strategic direction to the running of the two days themselves.",
    "convened_by": {
      "name": "Kerala Lifesciences Industries Parks",
      "note": "A subsidiary of Kerala State Industrial Development Corporation, under the Industries Department, Government of Kerala."
    },
    "people": [
      {"name":"V. D. Satheesan","role":"Hon'ble Chief Minister, Government of Kerala","badge":"Chief Minister","image_url":"https://bioconnect.kerala.gov.in/assets/leader-chief-minister.webp"},
      {"name":"P. K. Kunhalikutty","role":"Hon'ble Minister for Industries and Information Technology, Government of Kerala","badge":"Chairman, Advisory Committee","image_url":"https://bioconnect.kerala.gov.in/assets/leader-industries-minister.webp","image_credit":"Portrait: Shihab tharayil, CC BY-SA 4.0, cropped and toned; the adaptation is shared under the same licence.","image_credit_url":"https://commons.wikimedia.org/wiki/File:P._K._Kunhalikutty.jpg"}
    ],
    "committee": {
      "title": "Advisory Committee",
      "order_note": "Constituted by G.O.(Rt) No. 879/2026/ID, Industries Department, Government of Kerala, dated 03.08.2026.",
      "members": [
        {"role":"Chairman","name":"Hon'ble Minister for Industries","organization":"Government of Kerala"},
        {"role":"Member","name":"Additional Chief Secretary","organization":"Industries Department"},
        {"role":"Member","name":"Additional Chief Secretary","organization":"Health Department"},
        {"role":"Member","name":"Chairman","organization":"KSIDC / KLIP"},
        {"role":"Convenor","name":"Managing Director","organization":"KSIDC / KLIP"},
        {"role":"Member","name":"Board of Directors","organization":"KLIP"},
        {"role":"Member","name":"Executive Vice President","organization":"Kerala State Council for Science, Technology and Environment"},
        {"role":"Member","name":"Chairman & Managing Director","organization":"HLL Lifecare Ltd"},
        {"role":"Member","name":"Mr. Aju Jacob","organization":"Managing Director, Synthite Group"},
        {"role":"Member","name":"Mr. Thomas John","organization":"Managing Director, Agappe"},
        {"role":"Member","name":"Mr. John Kuriakose","organization":"Founder, DentCare"},
        {"role":"Member","name":"Director","organization":"Institute of Advanced Virology, Government of Kerala"},
        {"role":"Member","name":"Director","organization":"CSIR - National Institute for Interdisciplinary Science and Technology"},
        {"role":"Member","name":"Chief Executive Officer","organization":"KLIP"}
      ]
    }
  }
}
$content$::jsonb
WHERE id = 'mobile';
