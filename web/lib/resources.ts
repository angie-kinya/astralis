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
    label: "Kenya Red Cross counselling",
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

export type ReadingCategory = "health" | "science" | "work";

export type Article = {
  title: string;
  source: string;
  category: ReadingCategory;
  /** One sentence, in our words. */
  summary: string;
  /** Approximate, at about 230 words a minute. */
  minutes: number;
  url: string;
};

/**
 * Articles that open straight away: free, no paywall, no sign-up, no app.
 * Every URL was opened and read before it went in.
 */
export const ARTICLES: Article[] = [
  // Health and mind
  {
    title: "What to do when racing thoughts keep you up at night",
    source: "Psyche",
    category: "health",
    summary: "Fighting your thoughts or forcing sleep tends to backfire, so this guide offers gentler ways to put some distance between you and night-time worry.",
    minutes: 8,
    url: "https://psyche.co/guides/what-to-do-when-racing-thoughts-keep-you-up-at-night",
  },
  {
    title: "How to overcome insomnia by breaking your behaviour patterns",
    source: "Psyche",
    category: "health",
    summary: "Insomnia is very treatable, and changing your sleep habits works better than pills or potions.",
    minutes: 18,
    url: "https://psyche.co/guides/how-to-overcome-insomnia-by-breaking-your-behaviour-patterns",
  },
  {
    title: "Good Sleep for Good Health",
    source: "NIH News in Health",
    category: "health",
    summary: "Good sleep is about how long, how well and how regularly you sleep, and most adults need at least seven hours.",
    minutes: 4,
    url: "https://newsinhealth.nih.gov/2021/04/good-sleep-good-health",
  },
  {
    title: "Feeling Stressed?",
    source: "NIH News in Health",
    category: "health",
    summary: "Stress shows up in the body in ways you might not expect, and there are simple things that help you manage it.",
    minutes: 4,
    url: "https://newsinhealth.nih.gov/2021/01/feeling-stressed",
  },
  {
    title: "Mental health at work",
    source: "World Health Organization",
    category: "health",
    summary: "Heavy workloads, little control and insecure jobs put mental health at risk, and the WHO sets out what employers can do about it.",
    minutes: 4,
    url: "https://www.who.int/news-room/fact-sheets/detail/mental-health-at-work",
  },
  {
    title: "Why Africa needs to invest in mental health",
    source: "The Conversation Africa",
    category: "health",
    summary: "Many African governments spend less than one per cent of their health budgets on mental health, and Kenya has a single psychiatric hospital.",
    minutes: 4,
    url: "https://theconversation.com/why-africa-needs-to-invest-in-mental-health-147860",
  },

  // Science and nerdy stuff
  {
    title: "Sleeping Brain Waves Draw a Healthy Bath for Neurons",
    source: "Quanta Magazine",
    category: "science",
    summary: "In deep sleep, slow waves of brain activity, blood and spinal fluid pulse through the brain, which may help wash it clean.",
    minutes: 6,
    url: "https://www.quantamagazine.org/sleeping-brain-waves-draw-a-healthy-bath-for-neurons-20191216/",
  },
  {
    title: "Sleep Evolved Before Brains. Hydras Are Living Proof.",
    source: "Quanta Magazine",
    category: "science",
    summary: "Tiny animals with no brain at all still sleep, which suggests sleep began as something the body needed long before brains existed.",
    minutes: 11,
    url: "https://www.quantamagazine.org/sleep-evolved-before-brains-hydras-are-living-proof-20210518/",
  },
  {
    title: "How the Brain Moves From Waking Life to Sleep (and Back Again)",
    source: "Quanta Magazine",
    category: "science",
    summary: "Falling asleep is less like a switch and more like a series of in-between states, and sleep problems start when those steps go wrong.",
    minutes: 14,
    url: "https://www.quantamagazine.org/how-the-brain-moves-from-waking-life-to-sleep-and-back-again-20251017/",
  },
  {
    title: "Dueling Brain Waves Anchor or Erase Learning During Sleep",
    source: "Quanta Magazine",
    category: "science",
    summary: "While you sleep, one kind of slow brain wave strengthens the day's memories and another kind quietly weakens them.",
    minutes: 8,
    url: "https://www.quantamagazine.org/dueling-brain-waves-anchor-or-erase-learning-during-sleep-20191024/",
  },
  {
    title: "A fossil cranium from Kenya tells the story of an extinct elephant species",
    source: "The Conversation Africa",
    category: "science",
    summary: "A skull about 4.5 million years old, found near Lake Turkana, belonged to an elephant far larger than any alive today.",
    minutes: 5,
    url: "https://theconversation.com/a-fossil-cranium-from-kenya-tells-the-story-of-an-extinct-elephant-species-171960",
  },

  // Work and life
  {
    title: "ChatGPT is taking ghostwriters’ jobs in Kenya",
    source: "Rest of World",
    category: "work",
    summary: "Kenyan freelancers who write essays for students abroad are earning less as AI writing tools spread.",
    minutes: 5,
    url: "https://restofworld.org/2023/chatgpt-taking-kenya-ghostwriters-jobs/",
  },
  {
    title: "How Big Tech’s AI labor supply chain relies on hidden African workers",
    source: "Rest of World",
    category: "work",
    summary: "African content moderators are using European data laws to uncover who they really work for and to demand openness.",
    minutes: 3,
    url: "https://restofworld.org/2025/big-tech-ai-labor-supply-chain-african-workers/",
  },
  {
    title: "Catching a break: How gig workers find rest",
    source: "Rest of World",
    category: "work",
    summary: "More than a hundred gig workers in ten cities describe how they find rest between jobs.",
    minutes: 10,
    url: "https://restofworld.org/2024/gig-worker-rest-breaks/",
  },
  {
    title: "Why do identical informal businesses set up side by side? It’s a survival tactic",
    source: "The Conversation Africa",
    category: "work",
    summary: "Kenyan car repair businesses cluster together because neighbours become an informal safety net for one another.",
    minutes: 5,
    url: "https://theconversation.com/why-do-identical-informal-businesses-set-up-side-by-side-its-a-survival-tactic-kenya-study-225582",
  },
  {
    title: "What hunter-gatherers demonstrate about work and satisfaction",
    source: "Aeon",
    category: "work",
    summary: "Hunter-gatherers did not live in paradise, but the value they place on rest is a sharp contrast with modern working culture.",
    minutes: 20,
    url: "https://aeon.co/essays/what-hunter-gatherers-demonstrate-about-work-and-satisfaction",
  },
  {
    title: "Are we working more than ever?",
    source: "Our World in Data",
    category: "work",
    summary: "Working hours have fallen a long way over 150 years, although the gap between countries is still wide.",
    minutes: 7,
    url: "https://ourworldindata.org/working-more-than-ever",
  },
];

export type Fact = { text: string; source: string; url: string };

/** Each fact is stated on the page it links to. */
export const FACTS: Fact[] = [
  {
    text: "Your brain is about 2% of your body weight, yet it uses around 20% of your energy.",
    source: "The Conversation",
    url: "https://theconversation.com/how-much-energy-do-we-expend-thinking-and-using-our-brain-197990",
  },
  {
    text: "An adult brain runs on roughly 260 calories a day, which is about 0.3 kWh.",
    source: "The Conversation",
    url: "https://theconversation.com/how-much-energy-do-we-expend-thinking-and-using-our-brain-197990",
  },
  {
    text: "Stressful events can make your body burn fewer calories at rest.",
    source: "NIH News in Health",
    url: "https://newsinhealth.nih.gov/2021/01/feeling-stressed",
  },
  {
    text: "Working 55 hours a week or more was linked to 745,000 deaths from stroke and heart disease in 2016.",
    source: "World Health Organization",
    url: "https://www.who.int/news/item/17-05-2021-long-working-hours-increasing-deaths-from-heart-disease-and-stroke-who-ilo",
  },
  {
    text: "After 17 hours awake, you are about as impaired as someone with a blood alcohol level of 0.05%.",
    source: "CDC NIOSH",
    url: "https://www.cdc.gov/niosh/work-hour-training-for-nurses/longhours/mod3/08.html",
  },
  {
    text: "People spend nearly half their waking hours thinking about something other than what they are doing, and it tends to make them less happy.",
    source: "Harvard Gazette",
    url: "https://news.harvard.edu/gazette/story/2010/11/wandering-mind-not-a-happy-mind/",
  },
  {
    text: "A sugar cube of neutron star material would weigh about a billion tons on Earth. That is roughly a mountain.",
    source: "NASA",
    url: "https://imagine.gsfc.nasa.gov/science/objects/neutron_stars1.html",
  },
  {
    text: "A day on Venus lasts 243 Earth days. Its year lasts only 225.",
    source: "NASA",
    url: "https://science.nasa.gov/venus/venus-facts/",
  },
  {
    text: "An octopus has about as many neurons as a dog, and around two thirds of them are in its arms.",
    source: "Natural History Museum",
    url: "https://www.nhm.ac.uk/discover/octopuses-keep-surprising-us-here-are-eight-examples-how.html",
  },
  {
    text: "Tardigrades, also called water bears, survived ten days in the vacuum of space.",
    source: "European Space Agency",
    url: "https://www.esa.int/Science_Exploration/Human_and_Robotic_Exploration/Research/Tiny_animals_survive_exposure_to_space",
  },
  {
    text: "In the UK's four day week trial, 71% of staff reported less burnout, and 56 of the 61 companies chose to carry on.",
    source: "University of Cambridge",
    url: "https://www.sociology.cam.ac.uk/news/new-results-worlds-largest-trial-four-day-working-week",
  },
  {
    text: "The WHO does not class burnout as a medical condition. It calls it an occupational phenomenon caused by chronic workplace stress.",
    source: "World Health Organization",
    url: "https://www.who.int/news/item/28-05-2019-burn-out-an-occupational-phenomenon-international-classification-of-diseases",
  },
  {
    text: "Depression and anxiety cost the world about 12 billion working days a year.",
    source: "World Health Organization",
    url: "https://www.who.int/news-room/fact-sheets/detail/mental-health-at-work",
  },
  {
    text: "Hydras have no brain but still sleep. Dopamine, which keeps most animals awake, sends them to rest.",
    source: "Quanta Magazine",
    url: "https://www.quantamagazine.org/sleep-evolved-before-brains-hydras-are-living-proof-20210518/",
  },
  {
    text: "In deep sleep, a wave of spinal fluid washes up through your brain about three times a minute.",
    source: "Quanta Magazine",
    url: "https://www.quantamagazine.org/sleeping-brain-waves-draw-a-healthy-bath-for-neurons-20191216/",
  },
  {
    text: "Workers in richer countries tend to work fewer hours than workers in poorer ones.",
    source: "Our World in Data",
    url: "https://ourworldindata.org/rich-poor-working-hours",
  },
];

export const RESEARCH: Link[] = [
  {
    label: "WHO on burnout in ICD-11",
    detail: "The official definition: an occupational phenomenon, explicitly not a medical condition",
    href: "https://www.who.int/news/item/28-05-2019-burn-out-an-occupational-phenomenon-international-classification-of-diseases",
  },
  {
    label: "Maslach Burnout Inventory",
    detail: "The instrument most burnout research is built on. This free paper sets out its three scales: exhaustion, depersonalisation and accomplishment",
    href: "https://pmc.ncbi.nlm.nih.gov/articles/PMC7266903/",
  },
  {
    label: "OpenAlex",
    detail: "The open index Astralis reads from. Search it yourself; everything here is checkable",
    href: "https://openalex.org",
  },
  {
    label: "Occupational burnout on PubMed",
    detail: "Peer-reviewed literature, free abstracts",
    href: "https://pubmed.ncbi.nlm.nih.gov/?term=occupational+burnout",
  },
];

/** One phase of a breathing pacer. The ring eases to `scale` over `seconds`. */
export type PacePhase = { label: "Inhale" | "Hold" | "Exhale"; seconds: number; scale: number };

export type Practice = {
  name: string;
  minutes: string;
  why: string;
  steps: string[];
  /** Breathing practices only: drives the animated pacer. */
  pace?: PacePhase[];
};

/**
 * Things to actually do, with the mechanism stated. "Why" matters: people
 * abandon a practice they were told to do and keep one they understand.
 */
export const PRACTICES: Practice[] = [
  {
    name: "Physiological sigh",
    minutes: "60 seconds",
    // A long first inhale, a short second sniff on top, then an exhale about
    // twice as long as both. The steps give no counts, so these are typical.
    pace: [
      { label: "Inhale", seconds: 3, scale: 0.85 },
      { label: "Inhale", seconds: 1, scale: 1 },
      { label: "Exhale", seconds: 6, scale: 0.45 },
    ],
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
    pace: [
      { label: "Inhale", seconds: 4, scale: 1 },
      { label: "Hold", seconds: 4, scale: 1 },
      { label: "Exhale", seconds: 4, scale: 0.45 },
      { label: "Hold", seconds: 4, scale: 0.45 },
    ],
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
