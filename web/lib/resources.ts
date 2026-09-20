/**
 * Curated, real resources. Every link here was chosen deliberately; none is
 * generated, affiliate, or filler.
 *
 * Two rules for anything added to this file:
 *   1. Verify the link and — for helplines — verify the number by calling it.
 *      A wrong number on a crisis card is worse than no card at all.
 *   2. Nothing that claims to treat, diagnose, or replace care. This is a
 *      reading list and a set of things to try, not an intervention.
 */

export type Link = { label: string; detail: string; href: string };

export const HELPLINES: Link[] = [
  {
    label: "Befrienders Kenya",
    detail: "Free, confidential emotional support. +254 722 178 177",
    href: "tel:+254722178177",
  },
  {
    label: "Kenya Red Cross — counselling",
    detail: "Toll free, 24 hours. Dial 1199",
    href: "tel:1199",
  },
  {
    label: "Emergency services",
    detail: "Immediate danger. 999 or 112",
    href: "tel:999",
  },
  {
    label: "Find a Helpline",
    detail: "Free, confidential helplines in almost every country",
    href: "https://findahelpline.com",
  },
];

export const READING: Link[] = [
  {
    label: "Burnout: The Secret to Unlocking the Stress Cycle",
    detail: "Emily Nagoski & Amelia Nagoski — on completing the stress cycle, not just removing stressors",
    href: "https://openlibrary.org/search?q=Burnout+Nagoski",
  },
  {
    label: "Can't Even: How Millennials Became the Burnout Generation",
    detail: "Anne Helen Petersen — burnout as a structural condition rather than a personal failing",
    href: "https://openlibrary.org/search?q=Can%27t+Even+Anne+Helen+Petersen",
  },
  {
    label: "Rest: Why You Get More Done When You Work Less",
    detail: "Alex Soojung-Kim Pang — the research case that recovery is work, not its absence",
    href: "https://openlibrary.org/search?q=Rest+Alex+Soojung-Kim+Pang",
  },
  {
    label: "Four Thousand Weeks: Time Management for Mortals",
    detail: "Oliver Burkeman — on giving up the belief that you can get on top of everything",
    href: "https://openlibrary.org/search?q=Four+Thousand+Weeks+Burkeman",
  },
  {
    label: "Why We Sleep",
    detail: "Matthew Walker — sleep is the first thing burnout takes and the first thing to rebuild",
    href: "https://openlibrary.org/search?q=Why+We+Sleep+Matthew+Walker",
  },
];

export const RESEARCH: Link[] = [
  {
    label: "WHO — burnout in ICD-11",
    detail: "The official definition: an occupational phenomenon, explicitly not a medical condition",
    href: "https://www.who.int/news/item/28-05-2019-burn-out-an-occupational-phenomenon-international-classification-of-diseases",
  },
  {
    label: "Maslach Burnout Inventory",
    detail: "The instrument most burnout research is built on — three dimensions, not one score",
    href: "https://www.mindgarden.com/117-maslach-burnout-inventory-mbi",
  },
  {
    label: "OpenAlex",
    detail: "The open index Astralis reads from. Search it yourself; everything here is checkable",
    href: "https://openalex.org",
  },
  {
    label: "PubMed — occupational burnout",
    detail: "Peer-reviewed literature, free abstracts",
    href: "https://pubmed.ncbi.nlm.nih.gov/?term=occupational+burnout",
  },
];

export type Practice = {
  name: string;
  minutes: string;
  why: string;
  steps: string[];
};

/**
 * Things to actually do, with the mechanism stated. "Why" matters: people
 * abandon a practice they were told to do and keep one they understand.
 */
export const PRACTICES: Practice[] = [
  {
    name: "Physiological sigh",
    minutes: "60 seconds",
    why: "Two inhales then a long exhale offloads carbon dioxide faster than one breath can, which drops heart rate within a few cycles. The quickest reliable way down from acute stress.",
    steps: [
      "Inhale through the nose until the lungs feel nearly full.",
      "Without exhaling, take a second short sniff on top.",
      "Exhale slowly through the mouth until empty.",
      "Repeat three to five times.",
    ],
  },
  {
    name: "Box breathing",
    minutes: "3 minutes",
    why: "Equal counts give attention something dull and rhythmic to hold, which interrupts rumination. Used by people who need to think clearly under pressure.",
    steps: [
      "Inhale for a count of four.",
      "Hold for four.",
      "Exhale for four.",
      "Hold empty for four. Repeat for about three minutes.",
    ],
  },
  {
    name: "5-4-3-2-1 grounding",
    minutes: "2 minutes",
    why: "Anxiety runs on prediction. Naming what is actually present pulls attention out of the forecast and back into the room.",
    steps: [
      "Name five things you can see.",
      "Four you can feel touching you.",
      "Three you can hear.",
      "Two you can smell, and one you can taste.",
    ],
  },
  {
    name: "Detachment walk",
    minutes: "15 minutes",
    why: "Psychological detachment during off-hours is one of the most consistently supported predictors of recovery. Leaving the phone behind is the part that does the work.",
    steps: [
      "Leave the phone at home, or in a bag you will not open.",
      "Walk without a destination or a podcast.",
      "When work thoughts arrive, note them and return to what you can see.",
    ],
  },
  {
    name: "Shutdown ritual",
    minutes: "5 minutes",
    why: "An unfinished task keeps claiming attention until it has a plan. Writing tomorrow's first move gives your mind permission to stop holding it.",
    steps: [
      "Write down every open loop still in your head.",
      "Pick the single first action for tomorrow and write it as a verb.",
      "Say a fixed phrase out loud that marks the end of the day.",
      "Close the laptop. Do not reopen it.",
    ],
  },
  {
    name: "Progressive muscle release",
    minutes: "10 minutes",
    why: "Chronic stress holds tension you stop noticing. Deliberately tensing then releasing makes the contrast legible, and the release generalises.",
    steps: [
      "Start at the feet. Tense hard for five seconds.",
      "Release completely and notice the difference for ten seconds.",
      "Move upward: calves, thighs, hands, arms, shoulders, jaw, face.",
    ],
  },
];
