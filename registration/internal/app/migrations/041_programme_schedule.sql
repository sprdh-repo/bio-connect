-- The confirmed programme for 8-9 October 2026, from the organisers'
-- Programme, Inaugural and Valedictory schedules (received 2026-10-06).
-- Staff edit it afterwards in the console's Mobile app editor, under Sessions.
-- It only lands while nothing is published yet, so a programme staff have
-- started publishing is never overwritten; existing drafts stay, after it.
-- Names follow the speaker directory's spelling; designations are as printed.
UPDATE event_guide SET revision = revision + 1, updated_at = now(), document = jsonb_set(document, '{sessions}',
  $programme$
[
 {
  "id": "d1-registration",
  "kind": "break",
  "label": "",
  "title": "Registration",
  "track": "",
  "description": "",
  "starts_at": "2026-10-08T08:00:00+05:30",
  "ends_at": "2026-10-08T09:45:00+05:30",
  "location": "",
  "people": [],
  "segments": [],
  "speakers": "",
  "speaker_ids": [],
  "published": true
 },
 {
  "id": "d1-inaugural",
  "kind": "ceremony",
  "label": "",
  "title": "Inaugural Session & Exhibit Inauguration",
  "track": "",
  "description": "",
  "starts_at": "2026-10-08T10:00:00+05:30",
  "ends_at": "2026-10-08T11:40:00+05:30",
  "location": "Grand Ball Room",
  "people": [],
  "segments": [
   {
    "starts_at": "2026-10-08T10:00:00+05:30",
    "ends_at": "2026-10-08T10:03:00+05:30",
    "title": "Opening Remarks",
    "description": "",
    "people": [
     {
      "name": "Master of Ceremonies",
      "designation": "",
      "role": "",
      "speaker_id": ""
     }
    ]
   },
   {
    "starts_at": "2026-10-08T10:03:00+05:30",
    "ends_at": "2026-10-08T10:10:00+05:30",
    "title": "Welcome Address",
    "description": "",
    "people": [
     {
      "name": "Shri. Snehil Kumar Singh IAS",
      "designation": "Managing Director, KSIDC & KLIP; Director of Industries & Commerce and Managing Director, KSITIL, Government of Keralam",
      "role": "",
      "speaker_id": ""
     }
    ]
   },
   {
    "starts_at": "2026-10-08T10:10:00+05:30",
    "ends_at": "2026-10-08T10:20:00+05:30",
    "title": "Context Setting - Biotechnology & Lifesciences: Next Development Leap",
    "description": "",
    "people": [
     {
      "name": "Shri. A. P. M. Mohammed Hanish IAS",
      "designation": "Addl. Chief Secretary, Industries & Commerce, External Cooperation, Revenue (Waqf), Government of Keralam",
      "role": "",
      "speaker_id": ""
     }
    ]
   },
   {
    "starts_at": "2026-10-08T10:20:00+05:30",
    "ends_at": "2026-10-08T10:25:00+05:30",
    "title": "Launch of Vision AV - Shaping the Future of Life",
    "description": "Audio-visual presentation on the flagship project portfolio - Bio 360 Life Sciences Park.",
    "people": []
   },
   {
    "starts_at": "2026-10-08T10:25:00+05:30",
    "ends_at": "2026-10-08T10:40:00+05:30",
    "title": "Presidential Address",
    "description": "",
    "people": [
     {
      "name": "Shri. P. K. Kunhalikutty",
      "designation": "Hon'ble Minister for Industries, IT & AI, Government of Keralam",
      "role": "",
      "speaker_id": ""
     }
    ]
   },
   {
    "starts_at": "2026-10-08T10:40:00+05:30",
    "ends_at": "2026-10-08T10:50:00+05:30",
    "title": "Distinguished Address - Guest of Honour",
    "description": "",
    "people": [
     {
      "name": "Shri. K. Muraleedharan",
      "designation": "Hon'ble Minister for Health & Devaswoms, Government of Keralam",
      "role": "",
      "speaker_id": ""
     }
    ]
   },
   {
    "starts_at": "2026-10-08T10:50:00+05:30",
    "ends_at": "2026-10-08T11:00:00+05:30",
    "title": "Distinguished Address - Guest of Honour",
    "description": "",
    "people": [
     {
      "name": "Shri. T. Siddique",
      "designation": "Hon'ble Minister for Agriculture, Government of Keralam",
      "role": "",
      "speaker_id": ""
     }
    ]
   },
   {
    "starts_at": "2026-10-08T11:00:00+05:30",
    "ends_at": "2026-10-08T11:15:00+05:30",
    "title": "Inaugural Address - Next Development Leap",
    "description": "",
    "people": [
     {
      "name": "Shri. V. D. Satheesan",
      "designation": "Hon'ble Chief Minister, Government of Keralam",
      "role": "",
      "speaker_id": ""
     }
    ]
   },
   {
    "starts_at": "2026-10-08T11:15:00+05:30",
    "ends_at": "2026-10-08T11:20:00+05:30",
    "title": "Special Address",
    "description": "",
    "people": [
     {
      "name": "Dr. Jayakrishna Ambati",
      "designation": "Founding Director, Center for Advanced Vision Science (CAVS) & CEO, Inflammasomes, USA",
      "role": "",
      "speaker_id": "jayakrishna-ambati"
     }
    ]
   },
   {
    "starts_at": "2026-10-08T11:20:00+05:30",
    "ends_at": "2026-10-08T11:25:00+05:30",
    "title": "Honouring the Guests & EOI Receipts",
    "description": "",
    "people": []
   },
   {
    "starts_at": "2026-10-08T11:25:00+05:30",
    "ends_at": "2026-10-08T11:30:00+05:30",
    "title": "Special Address",
    "description": "",
    "people": [
     {
      "name": "Shri. C. Balagopal",
      "designation": "Chairman, KSIDC & KLIP, Government of Keralam",
      "role": "",
      "speaker_id": "c-balagopal"
     }
    ]
   },
   {
    "starts_at": "2026-10-08T11:30:00+05:30",
    "ends_at": "2026-10-08T11:35:00+05:30",
    "title": "Introducing Bio Connect 4.0 - Theme",
    "description": "",
    "people": [
     {
      "name": "Dr. C. N. Ramchand",
      "designation": "Director, KLIP & CEO, MagGenome Technologies",
      "role": "",
      "speaker_id": ""
     }
    ]
   },
   {
    "starts_at": "2026-10-08T11:35:00+05:30",
    "ends_at": "2026-10-08T11:40:00+05:30",
    "title": "Vote of Thanks",
    "description": "",
    "people": [
     {
      "name": "Shri. Jose Kurian Mundackal",
      "designation": "GM (i/c), KSIDC & Chief Executive Officer, KLIP",
      "role": "",
      "speaker_id": ""
     }
    ]
   },
   {
    "starts_at": "",
    "ends_at": "",
    "title": "Media interaction, if any",
    "description": "",
    "people": []
   },
   {
    "starts_at": "",
    "ends_at": "",
    "title": "Inaugural session concludes; the event commences",
    "description": "",
    "people": []
   }
  ],
  "speakers": "",
  "speaker_ids": [],
  "published": true
 },
 {
  "id": "d1-tea-1",
  "kind": "break",
  "label": "",
  "title": "Tea Break",
  "track": "",
  "description": "",
  "starts_at": "2026-10-08T11:40:00+05:30",
  "ends_at": "2026-10-08T11:50:00+05:30",
  "location": "",
  "people": [],
  "segments": [],
  "speakers": "",
  "speaker_ids": [],
  "published": true
 },
 {
  "id": "d1-keynote-ramasami",
  "kind": "talk",
  "label": "Keynote Address",
  "title": "Padma Bhushan Prof. T. Ramasami",
  "track": "",
  "description": "",
  "starts_at": "2026-10-08T11:50:00+05:30",
  "ends_at": "2026-10-08T12:05:00+05:30",
  "location": "",
  "people": [
   {
    "name": "Padma Bhushan Prof. T. Ramasami",
    "designation": "Former Secretary, Indian Science and Technology",
    "role": "",
    "speaker_id": "thirumalachari-ramasami"
   }
  ],
  "segments": [],
  "speakers": "",
  "speaker_ids": [],
  "published": true
 },
 {
  "id": "d1-keynote-hanish",
  "kind": "talk",
  "label": "Keynote Address",
  "title": "Shri. A. P. M. Mohammed Hanish IAS",
  "track": "",
  "description": "",
  "starts_at": "2026-10-08T12:05:00+05:30",
  "ends_at": "2026-10-08T12:20:00+05:30",
  "location": "",
  "people": [
   {
    "name": "Shri. A. P. M. Mohammed Hanish IAS",
    "designation": "Addl. Chief Secretary, Industries & Commerce, External Cooperation, Revenue (Waqf), Government of Keralam",
    "role": "",
    "speaker_id": ""
   }
  ],
  "segments": [],
  "speakers": "",
  "speaker_ids": [],
  "published": true
 },
 {
  "id": "d1-keynote-ambati",
  "kind": "talk",
  "label": "Keynote Address",
  "title": "Dr. Jayakrishna Ambati",
  "track": "",
  "description": "",
  "starts_at": "2026-10-08T12:20:00+05:30",
  "ends_at": "2026-10-08T12:35:00+05:30",
  "location": "",
  "people": [
   {
    "name": "Dr. Jayakrishna Ambati",
    "designation": "Founding Director, Center for Advanced Vision Science (CAVS); University of Virginia School of Medicine, USA; President and CEO, Inflammasomes",
    "role": "",
    "speaker_id": "jayakrishna-ambati"
   }
  ],
  "segments": [],
  "speakers": "",
  "speaker_ids": [],
  "published": true
 },
 {
  "id": "d1-keynote-minhaj-alam",
  "kind": "talk",
  "label": "Keynote Address",
  "title": "Shri. Minhaj Alam IAS",
  "track": "",
  "description": "",
  "starts_at": "2026-10-08T12:35:00+05:30",
  "ends_at": "2026-10-08T12:50:00+05:30",
  "location": "",
  "people": [
   {
    "name": "Shri. Minhaj Alam IAS",
    "designation": "Additional Chief Secretary, Agriculture & Commissioner, Agriculture Production, Industries (Coir) and Home & Vigilance, Government of Keralam",
    "role": "",
    "speaker_id": ""
   }
  ],
  "segments": [],
  "speakers": "",
  "speaker_ids": [],
  "published": true
 },
 {
  "id": "d1-keynote-snehil",
  "kind": "talk",
  "label": "Keynote Address",
  "title": "Shri. Snehil Kumar Singh IAS",
  "track": "",
  "description": "",
  "starts_at": "2026-10-08T12:50:00+05:30",
  "ends_at": "2026-10-08T13:05:00+05:30",
  "location": "",
  "people": [
   {
    "name": "Shri. Snehil Kumar Singh IAS",
    "designation": "Managing Director, KSIDC & KLIP; Director of Industries & Commerce and Managing Director, KSITIL, Government of Keralam",
    "role": "",
    "speaker_id": ""
   }
  ],
  "segments": [],
  "speakers": "",
  "speaker_ids": [],
  "published": true
 },
 {
  "id": "d1-lunch",
  "kind": "break",
  "label": "",
  "title": "Networking Lunch",
  "track": "",
  "description": "",
  "starts_at": "2026-10-08T13:05:00+05:30",
  "ends_at": "2026-10-08T13:50:00+05:30",
  "location": "",
  "people": [],
  "segments": [],
  "speakers": "",
  "speaker_ids": [],
  "published": true
 },
 {
  "id": "d1-keynote-miersch",
  "kind": "talk",
  "label": "Keynote Address",
  "title": "Dr. Shane Miersch",
  "track": "",
  "description": "",
  "starts_at": "2026-10-08T13:50:00+05:30",
  "ends_at": "2026-10-08T14:00:00+05:30",
  "location": "",
  "people": [
   {
    "name": "Dr. Shane Miersch",
    "designation": "CSO, Simisco Biologics, Canada",
    "role": "",
    "speaker_id": "shane-miersch"
   }
  ],
  "segments": [],
  "speakers": "",
  "speaker_ids": [],
  "published": true
 },
 {
  "id": "panel-1",
  "kind": "panel",
  "label": "Panel Discussion 1",
  "title": "Transforming Life Sciences Through Digital Innovation",
  "track": "AI",
  "description": "",
  "starts_at": "2026-10-08T14:00:00+05:30",
  "ends_at": "2026-10-08T14:40:00+05:30",
  "location": "",
  "people": [
   {
    "name": "Dr. Mahesh Iyer",
    "designation": "Head, Data and Qualitative Sciences, Bristol Myers Squibb",
    "role": "moderator",
    "speaker_id": "mahesh-iyer"
   },
   {
    "name": "Dr. Naveen Sivadasan",
    "designation": "Principal Scientist, Tata Consultancy Services (TCS)",
    "role": "panelist",
    "speaker_id": "naveen-sivadasan"
   },
   {
    "name": "Dr. Ajitesh Lunge",
    "designation": "CEO & Co-Founder, Locksmith Bio",
    "role": "panelist",
    "speaker_id": "ajitesh-harihar-lunge"
   },
   {
    "name": "Dr. Siddharth Joshi",
    "designation": "Director, Business Strategy, LTM",
    "role": "panelist",
    "speaker_id": "siddharth-m-joshi"
   },
   {
    "name": "Dr. Vinodh Kumar Adithya",
    "designation": "CEO, Capla GmbH",
    "role": "panelist",
    "speaker_id": "vinodh-kumar-adithya"
   }
  ],
  "segments": [],
  "speakers": "",
  "speaker_ids": [],
  "published": true
 },
 {
  "id": "panel-2",
  "kind": "panel",
  "label": "Panel Discussion 2",
  "title": "From Discovery to Next-Generation Healthcare",
  "track": "Biopharma",
  "description": "",
  "starts_at": "2026-10-08T14:40:00+05:30",
  "ends_at": "2026-10-08T15:30:00+05:30",
  "location": "",
  "people": [
   {
    "name": "Dr. C. S. Mani",
    "designation": "Surgical Oncologist, Apollo Hospital, Chennai",
    "role": "moderator",
    "speaker_id": "c-s-mani"
   },
   {
    "name": "Dr. Ravi Trivedi",
    "designation": "Vice President, Zydus Lifesciences Limited",
    "role": "panelist",
    "speaker_id": "ravi-trivedi"
   },
   {
    "name": "Dr. Preveen Ramamoorthy",
    "designation": "Chief Executive Officer, T Therapeutics LLC",
    "role": "panelist",
    "speaker_id": "preveen-ramamoorthy"
   },
   {
    "name": "Dr. Shane Miersch",
    "designation": "CSO, Simisco Biologics, Canada",
    "role": "panelist",
    "speaker_id": "shane-miersch"
   },
   {
    "name": "Dr. Rahul Purwar",
    "designation": "Professor, Indian Institute of Technology Bombay",
    "role": "panelist",
    "speaker_id": "rahul-purwar"
   }
  ],
  "segments": [],
  "speakers": "",
  "speaker_ids": [],
  "published": true
 },
 {
  "id": "d1-fireside",
  "kind": "panel",
  "label": "Fireside Chat",
  "title": "Beyond Generics: From Molecular Discovery to Global Innovation",
  "track": "",
  "description": "",
  "starts_at": "2026-10-08T15:30:00+05:30",
  "ends_at": "2026-10-08T15:55:00+05:30",
  "location": "",
  "people": [
   {
    "name": "Dr. Jayakrishna Ambati",
    "designation": "Founding Director, Centre for Advanced Vision Science (CAVS); University of Virginia School of Medicine, USA; President and CEO, Inflammasomes",
    "role": "",
    "speaker_id": "jayakrishna-ambati"
   },
   {
    "name": "Dr. C. N. Ramchand",
    "designation": "CEO & Founder, MagGenome Technologies",
    "role": "",
    "speaker_id": ""
   },
   {
    "name": "Ms. Vachas",
    "designation": "",
    "role": "host",
    "speaker_id": ""
   }
  ],
  "segments": [],
  "speakers": "",
  "speaker_ids": [],
  "published": true
 },
 {
  "id": "d1-tea-2",
  "kind": "break",
  "label": "",
  "title": "Tea Break",
  "track": "",
  "description": "",
  "starts_at": "2026-10-08T15:55:00+05:30",
  "ends_at": "2026-10-08T16:10:00+05:30",
  "location": "",
  "people": [],
  "segments": [],
  "speakers": "",
  "speaker_ids": [],
  "published": true
 },
 {
  "id": "d1-presentation-medtech",
  "kind": "talk",
  "label": "Presentation",
  "title": "The Rs. 40,000 Crore MedTech Opportunity for Keralam",
  "track": "",
  "description": "",
  "starts_at": "2026-10-08T16:10:00+05:30",
  "ends_at": "2026-10-08T16:20:00+05:30",
  "location": "",
  "people": [
   {
    "name": "Shri. C. Padmakumar",
    "designation": "Special Officer, Kerala Medical Technology Consortium",
    "role": "",
    "speaker_id": "c-padmakumar"
   }
  ],
  "segments": [],
  "speakers": "",
  "speaker_ids": [],
  "published": true
 },
 {
  "id": "panel-3",
  "kind": "panel",
  "label": "Panel Discussion 3",
  "title": "Realising the Kerala MedTech Vision 2032",
  "track": "",
  "description": "",
  "starts_at": "2026-10-08T16:10:00+05:30",
  "ends_at": "2026-10-08T16:55:00+05:30",
  "location": "",
  "people": [
   {
    "name": "Shri. C. Padmakumar",
    "designation": "Special Officer, KMTC",
    "role": "moderator",
    "speaker_id": "c-padmakumar"
   },
   {
    "name": "Shri. Balakrishnan T. P.",
    "designation": "Founder & CEO, i2R Labs",
    "role": "panelist",
    "speaker_id": "balakrishnan-t-p"
   },
   {
    "name": "Shri. T. C. Jayasankar",
    "designation": "General Secretary, KMDIA; MD, Jayon Implants Pvt. Ltd.",
    "role": "panelist",
    "speaker_id": "t-c-jayasankar"
   },
   {
    "name": "Shri. Joseph Jose",
    "designation": "Managing Director, Mariya Healthcare Pvt. Ltd.",
    "role": "panelist",
    "speaker_id": "joseph-jose"
   },
   {
    "name": "Shri. Binu Augustin",
    "designation": "Treasurer, KMDIA; Director, Heka Medicals India Pvt. Ltd.",
    "role": "panelist",
    "speaker_id": "binu-augustin"
   },
   {
    "name": "Shri. Sudhir Nair",
    "designation": "Vice President, Head Corporate Communications, Agappe Diagnostics Ltd.",
    "role": "panelist",
    "speaker_id": "sudhir-krishnan-nair"
   }
  ],
  "segments": [],
  "speakers": "",
  "speaker_ids": [],
  "published": true
 },
 {
  "id": "panel-4",
  "kind": "panel",
  "label": "Panel Discussion 4",
  "title": "The Trivandrum Medical Device Cluster",
  "track": "",
  "description": "",
  "starts_at": "2026-10-08T16:55:00+05:30",
  "ends_at": "2026-10-08T17:45:00+05:30",
  "location": "",
  "people": [
   {
    "name": "Shri. C. Balagopal",
    "designation": "Chairman, KSIDC & KLIP",
    "role": "moderator",
    "speaker_id": "c-balagopal"
   },
   {
    "name": "Dr. M. I. Sahadulla",
    "designation": "CMD, KIMS Health",
    "role": "panelist",
    "speaker_id": "m-i-sahadulla"
   },
   {
    "name": "Shri. Mukund R. Pillai",
    "designation": "Executive Director - Operations, HLL Lifecare",
    "role": "panelist",
    "speaker_id": "r-mukund"
   },
   {
    "name": "Shri. Sathish Kumar",
    "designation": "Managing Director, Terumo BCT",
    "role": "panelist",
    "speaker_id": "sathish-kumar-g"
   },
   {
    "name": "Shri. Praveen Sagar",
    "designation": "CTO / Technical Director, Ox Devices",
    "role": "panelist",
    "speaker_id": "praveen-sagar"
   }
  ],
  "segments": [],
  "speakers": "",
  "speaker_ids": [],
  "published": true
 },
 {
  "id": "d1-keynote-pappu",
  "kind": "talk",
  "label": "Keynote Address",
  "title": "Shri. Milind Pappu",
  "track": "",
  "description": "",
  "starts_at": "2026-10-08T17:45:00+05:30",
  "ends_at": "2026-10-08T18:05:00+05:30",
  "location": "",
  "people": [
   {
    "name": "Shri. Milind Pappu",
    "designation": "CEO, Nipro Medical India",
    "role": "",
    "speaker_id": "milind-pappu"
   }
  ],
  "segments": [],
  "speakers": "",
  "speaker_ids": [],
  "published": true
 },
 {
  "id": "panel-5",
  "kind": "panel",
  "label": "Panel Discussion 5",
  "title": "Innovation, Investment & Commercialisation of Medical Technologies",
  "track": "",
  "description": "",
  "starts_at": "2026-10-08T18:05:00+05:30",
  "ends_at": "2026-10-08T18:50:00+05:30",
  "location": "",
  "people": [
   {
    "name": "Dr. B. Satheesan",
    "designation": "Director, MCC",
    "role": "moderator",
    "speaker_id": "satheesan-b"
   },
   {
    "name": "Dr. Piyush Das",
    "designation": "Country Director & General Manager, Haier Biomedical, South Asia",
    "role": "panelist",
    "speaker_id": "piyush-das"
   },
   {
    "name": "Dr. Philip Mathew",
    "designation": "Director, Innovation & Partnerships, Believers Church Medical College Hospital",
    "role": "panelist",
    "speaker_id": "philip-mathew"
   },
   {
    "name": "Shri. Jay Krishnan",
    "designation": "Early-stage Investor, Partner, Beyond Next Ventures, India",
    "role": "panelist",
    "speaker_id": "jay-krishnan"
   },
   {
    "name": "Dr. Sreeja Narayanan",
    "designation": "CV Jacob Faculty & PI - Nano-Bioengineering, CUSAT",
    "role": "panelist",
    "speaker_id": "sreeja-narayanan"
   }
  ],
  "segments": [],
  "speakers": "",
  "speaker_ids": [],
  "published": true
 },
 {
  "id": "d1-gala",
  "kind": "social",
  "label": "",
  "title": "Cultural Program, Gala Dinner & Entertainment",
  "track": "",
  "description": "Featuring national and international musical instruments.",
  "starts_at": "2026-10-08T19:00:00+05:30",
  "ends_at": "2026-10-08T22:00:00+05:30",
  "location": "",
  "people": [],
  "segments": [],
  "speakers": "",
  "speaker_ids": [],
  "published": true
 },
 {
  "id": "d2-opening",
  "kind": "talk",
  "label": "",
  "title": "Opening Remarks",
  "track": "",
  "description": "",
  "starts_at": "2026-10-09T09:00:00+05:30",
  "ends_at": "2026-10-09T09:10:00+05:30",
  "location": "",
  "people": [
   {
    "name": "Shri. Jose Kurian Mundackal",
    "designation": "GM, KSIDC & CEO, KLIP",
    "role": "",
    "speaker_id": ""
   }
  ],
  "segments": [],
  "speakers": "",
  "speaker_ids": [],
  "published": true
 },
 {
  "id": "d2-keynote-pillai",
  "kind": "talk",
  "label": "Keynote Address",
  "title": "Dr. Beena Pillai",
  "track": "",
  "description": "",
  "starts_at": "2026-10-09T09:10:00+05:30",
  "ends_at": "2026-10-09T09:20:00+05:30",
  "location": "",
  "people": [
   {
    "name": "Dr. Beena Pillai",
    "designation": "Director, RGCB, Thiruvananthapuram",
    "role": "",
    "speaker_id": "beena-pillai"
   }
  ],
  "segments": [],
  "speakers": "",
  "speaker_ids": [],
  "published": true
 },
 {
  "id": "panel-6",
  "kind": "panel",
  "label": "Panel Discussion 6",
  "title": "Next-Gen Biologics: From Discovery to Market",
  "track": "Future Healthcare & Cancer",
  "description": "Trends in biologic drug development, manufacturing scale-up and regulatory pathways.",
  "starts_at": "2026-10-09T09:20:00+05:30",
  "ends_at": "2026-10-09T10:00:00+05:30",
  "location": "",
  "people": [
   {
    "name": "Dr. Aju Mathew",
    "designation": "Oncologist, Medical Centre, Kerala Cancer Care, Ernakulam",
    "role": "moderator",
    "speaker_id": "aju-mathew"
   },
   {
    "name": "Shri. Murali Neelakantan",
    "designation": "Principal Lawyer, Amicus",
    "role": "panelist",
    "speaker_id": "murali-neelakantan"
   },
   {
    "name": "Dr. Girish Chundayil Madathil",
    "designation": "Co-founder & CEO, Luxmatra Innovations Pvt. Ltd.",
    "role": "panelist",
    "speaker_id": "girish-chundayil-madathil"
   },
   {
    "name": "Prof. Ron Geyer",
    "designation": "Professor, Department of Pathology and Laboratory Medicine, University of Saskatchewan, Canada",
    "role": "panelist",
    "speaker_id": "ron-geyer"
   }
  ],
  "segments": [],
  "speakers": "",
  "speaker_ids": [],
  "published": true
 },
 {
  "id": "panel-7",
  "kind": "panel",
  "label": "Panel Discussion 7",
  "title": "From Kerala's Heritage to a Global Investment Opportunity",
  "track": "",
  "description": "",
  "starts_at": "2026-10-09T10:00:00+05:30",
  "ends_at": "2026-10-09T11:00:00+05:30",
  "location": "",
  "people": [
   {
    "name": "Dr. Vaidya Prasad",
    "designation": "Director & Chief Physician, Sunethri Ayurvedashram & Research Centre",
    "role": "moderator",
    "speaker_id": "vaidya-m-prasad"
   },
   {
    "name": "Dr. Sanjeev Kumar",
    "designation": "Former Senior Research Officer, DSU, Government Ayurveda College",
    "role": "panelist",
    "speaker_id": "p-sanjeev-kumar"
   },
   {
    "name": "Dr. D. G. Namboothiri",
    "designation": "General Manager of Research and Development (R&D), Sreedhareeyam Ayurvedic Eye Hospital & Research Centre",
    "role": "panelist",
    "speaker_id": "d-g-namboothiri"
   },
   {
    "name": "Dr. Ashish G. R.",
    "designation": "Principal Scientist, Vaidyaratnam",
    "role": "panelist",
    "speaker_id": "ashish-g-r"
   },
   {
    "name": "Dr. Sujith Eranezhath",
    "designation": "Chief Executive Officer, Incubation Centre for Ayurveda Innovation and Entrepreneurship",
    "role": "panelist",
    "speaker_id": "sujith-subash-eranezhath"
   },
   {
    "name": "Dr. Yadu Mooss",
    "designation": "Executive Director, Vaidyaratnam Group",
    "role": "panelist",
    "speaker_id": "yadu-narayanan-mooss"
   },
   {
    "name": "Dr. Rajmohan V.",
    "designation": "Associate Professor, Government Ayurveda College",
    "role": "panelist",
    "speaker_id": "rajmohan-v"
   }
  ],
  "segments": [],
  "speakers": "",
  "speaker_ids": [],
  "published": true
 },
 {
  "id": "d2-tea-1",
  "kind": "break",
  "label": "",
  "title": "Tea Break",
  "track": "",
  "description": "",
  "starts_at": "2026-10-09T11:00:00+05:30",
  "ends_at": "2026-10-09T11:10:00+05:30",
  "location": "",
  "people": [],
  "segments": [],
  "speakers": "",
  "speaker_ids": [],
  "published": true
 },
 {
  "id": "panel-8",
  "kind": "panel",
  "label": "Panel Discussion 8",
  "title": "Nutraceuticals: Unlocking Kerala's Next Wave of Health and Wellness Business",
  "track": "",
  "description": "",
  "starts_at": "2026-10-09T11:10:00+05:30",
  "ends_at": "2026-10-09T12:00:00+05:30",
  "location": "",
  "people": [
   {
    "name": "Prof. G. M. Nair",
    "designation": "Director, Kerala Academy of Sciences (KAS)",
    "role": "moderator",
    "speaker_id": "g-m-nair"
   },
   {
    "name": "Dr. Jayashankar Das",
    "designation": "Founder & CEO, Elmentoz Research",
    "role": "panelist",
    "speaker_id": "jayashankar-das"
   },
   {
    "name": "Dr. Sreeraj Gopi",
    "designation": "Founder and Managing Director, Molecules Biolabs",
    "role": "panelist",
    "speaker_id": ""
   },
   {
    "name": "Dr. Sreekumar",
    "designation": "Chairman & Director, Wellness Solutions",
    "role": "panelist",
    "speaker_id": "sreekumar-appukuttan-nair"
   },
   {
    "name": "Dr. Vinaya K. K.",
    "designation": "R&D Head, WellGenome Pvt. Ltd.",
    "role": "panelist",
    "speaker_id": "vinaya-k-k"
   }
  ],
  "segments": [],
  "speakers": "",
  "speaker_ids": [],
  "published": true
 },
 {
  "id": "panel-9",
  "kind": "panel",
  "label": "Panel Discussion 9",
  "title": "Make in India: Transforming Innovation into Manufacturing, Markets and Economic Growth",
  "track": "",
  "description": "",
  "starts_at": "2026-10-09T12:00:00+05:30",
  "ends_at": "2026-10-09T12:30:00+05:30",
  "location": "",
  "people": [
   {
    "name": "Dr. Raj Shirumalla",
    "designation": "Senior Scientific Advisor, Govt. of India",
    "role": "moderator",
    "speaker_id": "raj-shirumalla"
   },
   {
    "name": "Dr. Aniruddha Bhati",
    "designation": "Principal Scientist & Business Lead, DSS Takara Biosciences, India",
    "role": "panelist",
    "speaker_id": "aniruddha-bhati"
   },
   {
    "name": "Dr. Chandrasekharan",
    "designation": "Founder, Chairman & MD, Vipragen Biosciences Pvt. Ltd, India",
    "role": "panelist",
    "speaker_id": "s-chandrashekaran"
   },
   {
    "name": "Shri. Harkaran Dhingra",
    "designation": "Chief Executive Officer, NeoDX, India",
    "role": "panelist",
    "speaker_id": "harkaran-dhingra"
   },
   {
    "name": "Prof. T. P. Singh",
    "designation": "Professor, AIIMS",
    "role": "panelist",
    "speaker_id": "t-p-singh"
   }
  ],
  "segments": [],
  "speakers": "",
  "speaker_ids": [],
  "published": true
 },
 {
  "id": "panel-10",
  "kind": "panel",
  "label": "Panel Discussion 10",
  "title": "Kerala's Bioeconomy: Building a Sustainable Future Through Innovation and Investment",
  "track": "",
  "description": "",
  "starts_at": "2026-10-09T12:30:00+05:30",
  "ends_at": "2026-10-09T13:20:00+05:30",
  "location": "",
  "people": [
   {
    "name": "Dr. C. N. Ramchand",
    "designation": "CEO, MedScape Pharma & MagGenome Technologies Pvt. Ltd.",
    "role": "moderator",
    "speaker_id": ""
   },
   {
    "name": "Shri. Bijoy Nair",
    "designation": "Co-Founder, President & CEO, Nord Metallics; Co-founder Director & Chief Strategy Officer, Bharat Biome / Indus Biome Life Sciences Pvt. Ltd.",
    "role": "panelist",
    "speaker_id": ""
   },
   {
    "name": "Shri. Balasubramanya S.",
    "designation": "Consultant - Strategy & Business Development, ABLE",
    "role": "panelist",
    "speaker_id": ""
   },
   {
    "name": "Dr. Rijesh Krishna",
    "designation": "Chief Operating Officer, IPTIF",
    "role": "panelist",
    "speaker_id": ""
   },
   {
    "name": "Shri. Ramjee Pallela",
    "designation": "Director, EY",
    "role": "panelist",
    "speaker_id": ""
   },
   {
    "name": "Dr. Ampady",
    "designation": "CEO, KRIBS & BioNest",
    "role": "panelist",
    "speaker_id": "k-ampady"
   }
  ],
  "segments": [],
  "speakers": "",
  "speaker_ids": [],
  "published": true
 },
 {
  "id": "d2-lunch",
  "kind": "break",
  "label": "",
  "title": "Networking Lunch",
  "track": "",
  "description": "",
  "starts_at": "2026-10-09T13:20:00+05:30",
  "ends_at": "2026-10-09T14:00:00+05:30",
  "location": "",
  "people": [],
  "segments": [],
  "speakers": "",
  "speaker_ids": [],
  "published": true
 },
 {
  "id": "d2-keynote-geyer",
  "kind": "talk",
  "label": "Keynote Address",
  "title": "Prof. Ron Geyer",
  "track": "",
  "description": "",
  "starts_at": "2026-10-09T14:00:00+05:30",
  "ends_at": "2026-10-09T14:15:00+05:30",
  "location": "",
  "people": [
   {
    "name": "Prof. Ron Geyer",
    "designation": "Professor, Department of Pathology and Laboratory Medicine, University of Saskatchewan, Canada",
    "role": "",
    "speaker_id": "ron-geyer"
   }
  ],
  "segments": [],
  "speakers": "",
  "speaker_ids": [],
  "published": true
 },
 {
  "id": "panel-11",
  "kind": "panel",
  "label": "Panel Discussion 11",
  "title": "Innovations for Food Security and Sustainability",
  "track": "Agrifood",
  "description": "",
  "starts_at": "2026-10-09T14:15:00+05:30",
  "ends_at": "2026-10-09T15:05:00+05:30",
  "location": "",
  "people": [
   {
    "name": "Dr. Meenakumari",
    "designation": "Former Deputy Director General (Fisheries), ICAR",
    "role": "moderator",
    "speaker_id": "b-meenakumari"
   },
   {
    "name": "Shri. S. K. Menon",
    "designation": "President, Q Life Consumer Products Pvt. Ltd., a NeST Group company",
    "role": "panelist",
    "speaker_id": "s-k-menon"
   },
   {
    "name": "Shri. Venkatachalam",
    "designation": "Founder & CEO, Certitude Farms",
    "role": "panelist",
    "speaker_id": "venkatachalam"
   },
   {
    "name": "Shri. Harshvardhan Joshi",
    "designation": "CEO & Director, Khadkeshwara Farms and Food Pvt. Ltd.",
    "role": "panelist",
    "speaker_id": ""
   },
   {
    "name": "Shri. Raj Dash",
    "designation": "Co-founder, Sr. Vice President, Strategic Initiatives; Co-founder, MD & CEO, Bharat Biome / Indus Biome Life Sciences Pvt. Ltd.",
    "role": "panelist",
    "speaker_id": ""
   }
  ],
  "segments": [],
  "speakers": "",
  "speaker_ids": [],
  "published": true
 },
 {
  "id": "d2-keynote-sharmila",
  "kind": "talk",
  "label": "Keynote Address",
  "title": "Dr. Sharmila Mary Joseph IAS",
  "track": "",
  "description": "",
  "starts_at": "2026-10-09T15:05:00+05:30",
  "ends_at": "2026-10-09T15:25:00+05:30",
  "location": "",
  "people": [
   {
    "name": "Dr. Sharmila Mary Joseph IAS",
    "designation": "Principal Secretary, Health & Family Welfare, Ayush, Food & Civil Supplies and Consumer Affairs Department",
    "role": "",
    "speaker_id": ""
   }
  ],
  "segments": [],
  "speakers": "",
  "speaker_ids": [],
  "published": true
 },
 {
  "id": "d2-product-launch",
  "kind": "",
  "label": "",
  "title": "Product Launch",
  "track": "",
  "description": "",
  "starts_at": "2026-10-09T15:25:00+05:30",
  "ends_at": "2026-10-09T16:25:00+05:30",
  "location": "",
  "people": [],
  "segments": [],
  "speakers": "",
  "speaker_ids": [],
  "published": true
 },
 {
  "id": "d2-tea",
  "kind": "break",
  "label": "",
  "title": "Tea Break",
  "track": "",
  "description": "",
  "starts_at": "2026-10-09T16:25:00+05:30",
  "ends_at": "2026-10-09T16:45:00+05:30",
  "location": "",
  "people": [],
  "segments": [],
  "speakers": "",
  "speaker_ids": [],
  "published": true
 },
 {
  "id": "d2-valedictory",
  "kind": "ceremony",
  "label": "",
  "title": "Valedictory Session",
  "track": "",
  "description": "",
  "starts_at": "2026-10-09T16:45:00+05:30",
  "ends_at": "2026-10-09T18:00:00+05:30",
  "location": "Grand Ball Room",
  "people": [],
  "segments": [
   {
    "starts_at": "2026-10-09T16:45:00+05:30",
    "ends_at": "2026-10-09T16:48:00+05:30",
    "title": "Opening Remarks",
    "description": "",
    "people": [
     {
      "name": "Master of Ceremonies",
      "designation": "",
      "role": "",
      "speaker_id": ""
     }
    ]
   },
   {
    "starts_at": "2026-10-09T16:48:00+05:30",
    "ends_at": "2026-10-09T17:00:00+05:30",
    "title": "Welcome Address and Key Outcomes of Bio Connect 4.0",
    "description": "",
    "people": [
     {
      "name": "Shri. Snehil Kumar Singh IAS",
      "designation": "Managing Director, KSIDC & KLIP; Director of Industries & Commerce and Managing Director, KSITIL, Government of Keralam",
      "role": "",
      "speaker_id": ""
     }
    ]
   },
   {
    "starts_at": "2026-10-09T17:00:00+05:30",
    "ends_at": "2026-10-09T17:10:00+05:30",
    "title": "Special Address - Global & Government Perspective",
    "description": "",
    "people": [
     {
      "name": "Shri. C. Balagopal",
      "designation": "Chairman, KSIDC & KLIP, Government of Keralam",
      "role": "",
      "speaker_id": "c-balagopal"
     }
    ]
   },
   {
    "starts_at": "2026-10-09T17:10:00+05:30",
    "ends_at": "2026-10-09T17:15:00+05:30",
    "title": "Scientific Sessions: Key Insights & Takeaways",
    "description": "",
    "people": [
     {
      "name": "Dr. C. N. Ramchand",
      "designation": "Director, KLIP & CEO, MagGenome Technologies",
      "role": "",
      "speaker_id": ""
     }
    ]
   },
   {
    "starts_at": "2026-10-09T17:15:00+05:30",
    "ends_at": "2026-10-09T17:20:00+05:30",
    "title": "Felicitation Address",
    "description": "",
    "people": [
     {
      "name": "Shri. Santhosh Koshy Thomas",
      "designation": "Director, KLIP & Managing Director, KINFRA",
      "role": "",
      "speaker_id": ""
     }
    ]
   },
   {
    "starts_at": "2026-10-09T17:20:00+05:30",
    "ends_at": "2026-10-09T17:25:00+05:30",
    "title": "Bio Connect 4.0 - Industry Perspectives",
    "description": "",
    "people": [
     {
      "name": "Dr. Sreeraj Gopi",
      "designation": "Founder and Managing Director, Molecules Biolabs",
      "role": "",
      "speaker_id": ""
     }
    ]
   },
   {
    "starts_at": "2026-10-09T17:25:00+05:30",
    "ends_at": "2026-10-09T17:30:00+05:30",
    "title": "Special Address - Investment Facilitation in Health Sciences",
    "description": "",
    "people": [
     {
      "name": "Shri. C. Padmakumar",
      "designation": "Director, KLIP & Special Officer, Kerala Medical Technology Consortium",
      "role": "",
      "speaker_id": "c-padmakumar"
     }
    ]
   },
   {
    "starts_at": "2026-10-09T17:30:00+05:30",
    "ends_at": "2026-10-09T17:40:00+05:30",
    "title": "The Way Forward - From Commitments to Groundbreaking",
    "description": "",
    "people": [
     {
      "name": "Shri. A. P. M. Mohammed Hanish IAS",
      "designation": "Addl. Chief Secretary, Industries & Commerce, External Cooperation, Revenue (Waqf), Government of Keralam",
      "role": "",
      "speaker_id": ""
     }
    ]
   },
   {
    "starts_at": "2026-10-09T17:40:00+05:30",
    "ends_at": "2026-10-09T17:55:00+05:30",
    "title": "Valedictory Address",
    "description": "",
    "people": [
     {
      "name": "Shri. P. K. Kunhalikutty",
      "designation": "Hon'ble Minister for Industries, IT & AI, Government of Keralam",
      "role": "",
      "speaker_id": ""
     }
    ]
   },
   {
    "starts_at": "2026-10-09T17:55:00+05:30",
    "ends_at": "2026-10-09T18:00:00+05:30",
    "title": "Vote of Thanks and Official Closing of Bio Connect 4.0",
    "description": "",
    "people": [
     {
      "name": "Shri. Varghese Malakkaran",
      "designation": "General Manager (IF&IP and HR), KSIDC, Government of Keralam",
      "role": "",
      "speaker_id": ""
     }
    ]
   },
   {
    "starts_at": "",
    "ends_at": "",
    "title": "Event concludes",
    "description": "",
    "people": []
   }
  ],
  "speakers": "",
  "speaker_ids": [],
  "published": true
 }
]
  $programme$::jsonb || coalesce(document->'sessions', '[]'::jsonb))
WHERE id = 'mobile'
  AND NOT EXISTS (SELECT 1 FROM jsonb_array_elements(coalesce(document->'sessions', '[]'::jsonb)) s
                  WHERE (s->>'published')::boolean);
