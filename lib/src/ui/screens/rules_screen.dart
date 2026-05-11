import 'package:flutter/material.dart';

import '../../domain/models.dart';
import '../components/zen_components.dart';

extension _RulesetTagline on Ruleset {
  String get tagline {
    switch (this) {
      case Ruleset.chinese:
        return 'Area scoring · 7.5 komi · positional superko';
      case Ruleset.japanese:
        return 'Territory + prisoners · 6.5 komi · basic ko';
      case Ruleset.korean:
        return 'Territory + prisoners · 6.5 komi · basic ko';
      case Ruleset.aga:
        return 'Area or territory · 7.5 komi · situational superko';
      case Ruleset.ing:
        return 'Area + fill-in · 8 komi · Ing superko · suicide legal';
      case Ruleset.newZealand:
        return 'Area scoring · 7 komi · situational superko · suicide legal';
      case Ruleset.trompTaylor:
        return 'Area scoring · configurable komi · suicide legal (engine ruleset)';
    }
  }
}

class _RuleSection {
  final String title;
  final List<String> rules;
  final String? note;
  const _RuleSection({required this.title, required this.rules, this.note});
}

const _commonSections = <_RuleSection>[
  _RuleSection(
    title: 'Equipment & setup',
    rules: [
      'Board: Standard play uses a 19×19 grid. 13×13 and 9×9 boards are used for teaching and faster games. The lines form 361 intersections on a 19×19 board.',
      'Stones: One player uses black stones, the other uses white stones. Stones are flat and lens-shaped. Each set conventionally contains 181 black and 180 white stones (enough to fill the board).',
      'Star points (hoshi): Nine intersections on a 19×19 board are marked with dots. These serve as visual reference points and as handicap stone placement positions.',
      'Komi: Because Black moves first and has a first-move advantage, White receives a points bonus called komi. The exact komi value depends on the ruleset (see Variation rules below). Most modern rulesets use a half-integer komi to eliminate ties.',
    ],
  ),
  _RuleSection(
    title: 'Objective of the game',
    rules: [
      'The goal is to control more of the board than your opponent by surrounding territory and capturing enemy stones.',
      'Territory is defined as empty intersections completely surrounded by one player\'s stones. Captured stones may or may not factor into the score depending on the ruleset (see Variation rules below).',
      'The game is scored when both players pass consecutively. The player with the higher score wins. Komi ensures the result is decided by a precise margin.',
    ],
  ),
  _RuleSection(
    title: 'Placing stones',
    rules: [
      'Black moves first. Players alternate turns. On your turn you must either place one stone on any unoccupied intersection, or pass.',
      'Stones are placed on intersections (the crossing points of lines), not inside squares.',
      'Once placed, stones do not move. They remain on their intersection unless captured.',
      'A move must satisfy the ruleset\'s Ko and suicide constraints. Both vary by ruleset (see Variation rules below).',
      'Passing: A player may pass their turn at any time. Some rulesets attach a pass-stone procedure to a pass (see Variation rules below). Two consecutive passes end the game in every ruleset.',
    ],
  ),
  _RuleSection(
    title: 'Liberties & capture',
    rules: [
      'Liberties are the empty intersections directly adjacent (up, down, left, right — never diagonal) to a stone or a connected group of stones.',
      'When all liberties of a stone or group are occupied by the opponent\'s stones, that stone or group is captured and immediately removed from the board.',
      'Capture is resolved at the moment a stone is placed. If your placed stone removes the last liberty of an enemy group, that group comes off the board before any other legality check.',
      'Atari is the state when a stone or group has exactly one liberty remaining. It is in immediate danger of capture. There is no obligation to announce Atari.',
      'Whether captured stones (prisoners) affect the final score depends on the ruleset (see Variation rules below).',
    ],
  ),
  _RuleSection(
    title: 'Groups & connectivity',
    rules: [
      'Two stones of the same color that are orthogonally adjacent form a group (also called a string or chain). Two stones connected indirectly through a chain of adjacent same-color stones are part of the same group.',
      'Connectivity is orthogonal only. Diagonally adjacent stones of the same color are NOT part of the same group.',
      'All stones in a group share the same set of liberties. If any liberty of the group is occupied by an enemy stone, every stone in the group loses that liberty.',
      'A group is captured only when all its liberties are occupied. You cannot partially capture a group.',
    ],
  ),
  _RuleSection(
    title: 'Basic Ko rule',
    rules: [
      'Ko (劫) prevents an infinite loop. The basic rule states: a player may not make a move that returns the board to the exact same position as it was immediately before the opponent\'s last move.',
      'This most commonly occurs in a "Ko fight" — a specific one-stone pattern where each player could repeatedly recapture the other\'s stone, cycling indefinitely without the Ko rule.',
      'After a Ko capture, the opponent must play elsewhere (a "Ko threat") before recapturing. If there is no suitable Ko threat, the Ko is abandoned.',
      'Some rulesets extend this with stricter superko rules that look further back than just the previous move. These resolve unusual cyclic positions like triple ko, eternal life, or approach-move ko (see Variation rules below).',
    ],
    note:
        'The basic ko rule alone is enough for almost all real games. Superko rules become relevant only in rare cyclic positions; how those are resolved is where rulesets diverge.',
  ),
  _RuleSection(
    title: 'Life & death basics',
    rules: [
      'A group is alive if it cannot be captured regardless of how the opponent plays. A group is dead if it cannot avoid eventual capture regardless of how it plays.',
      'Two eyes: The most common way to achieve life is for a group to contain two separate internal empty spaces (eyes) that the opponent cannot simultaneously fill. A group with two genuine eyes cannot be captured.',
      'False eye: An eye-shaped space is false if the opponent can eventually destroy it by playing on the key intersection that connects the surrounding stones. False eyes do not confer life.',
      'Seki (mutual life / impasse): Two opposing groups may share liberties such that neither player can capture the other without first losing their own group. Both groups live without having two eyes.',
      'Specific edge cases — bent-four-in-the-corner, ko-based life, how seki territory is scored — are handled differently across rulesets (see Variation rules below).',
    ],
  ),
  _RuleSection(
    title: 'End of the game',
    rules: [
      'The game ends when both players pass consecutively. This signals that neither player believes any further moves will increase their score.',
      'Before scoring, players must agree on which stones on the board are dead. Dead stones are removed before counting.',
      'If players disagree on the status of a group, they resume play to resolve the dispute. The player claiming a group is dead must prove it by capturing those stones in resumed play; the other player defends.',
      'Whether a pass during resumed play has a cost (transfers a point to the opponent) varies by ruleset (see Variation rules below).',
      'Resignation: a player may resign at any time, conceding the game without scoring. This is the normal way a clearly lost professional game ends.',
    ],
  ),
  _RuleSection(
    title: 'Handicap games',
    rules: [
      'Handicap stones allow players of different strengths to play competitively. The weaker player (Black) places extra stones on the board before White makes any move.',
      'Handicap stones are placed on the star points (hoshi) in a fixed pattern. On a 19×19 board, standard placement positions are: 2 stones = D4 and Q16; 3 stones = add D16; 4 = add Q4; 5 to 9 stones add centre and side star points in a traditional order.',
      'In a handicap game, White moves first (since Black has already "moved" by placing handicap stones).',
      'Komi in handicap games is typically reduced — often to 0.5 — to avoid doubly compensating White. The exact handicap-komi convention varies by ruleset.',
    ],
  ),
  _RuleSection(
    title: 'Rules of conduct & etiquette',
    rules: [
      'Resignation: A player may resign at any time, conceding the game. Resignation is the normal way professional games end. By convention, a player who resigns after a long game should not request to count the score.',
      'Stone placement: Stones must be placed decisively with a clean click. In formal play, once a stone touches the board it must be played at that intersection (no adjusting).',
      'Review after game: It is customary (and required in teaching contexts) to replay the game from memory after it ends to discuss key moments. Both players are expected to remember the game.',
      'Time: In timed games, a player who exceeds their time limit loses immediately. Byo-yomi (overtime) is common. The player must complete each move within a fixed period (e.g., 30 seconds) or lose.',
      'No take-backs: Once placed, a stone cannot be moved or removed by the player who placed it, except as a captured stone by the opponent.',
    ],
  ),
];

const _variationSections = <Ruleset, List<_RuleSection>>{
  Ruleset.chinese: _chineseSections,
  Ruleset.japanese: _japaneseSections,
  Ruleset.korean: _koreanSections,
  Ruleset.aga: _agaSections,
  Ruleset.ing: _ingSections,
  Ruleset.newZealand: _nzSections,
  Ruleset.trompTaylor: _trompTaylorSections,
};

const _chineseSections = <_RuleSection>[
  _RuleSection(
    title: 'Chinese — komi & scoring',
    rules: [
      'Komi: 7.5 points (modern professional standard). Older Chinese rules historically used 5.5.',
      'Area scoring: each player\'s score = (number of their living stones on the board) + (number of empty intersections completely surrounded by their stones).',
      'Captured stones (prisoners) do NOT count. They are kept aside but have no effect on the final score.',
      'Dame: empty points bordering both colours do not count for either player; in tournament play they are often filled to simplify counting.',
      'Equivalence: on a fully filled board, Chinese area scoring and Japanese territory scoring give the same winner. They diverge only when there are dame, pass, or bent-four disputes.',
      'Practical counting: one player\'s stones and territory are rearranged into a rectangle, then compared against 180.5 (half of 361 adjusted for komi).',
    ],
    note:
        'The Chinese ruleset is also called "area scoring" or "territory + stones scoring." It is standard in China and increasingly common internationally due to its simplicity in resolving life/death disputes.',
  ),
  _RuleSection(
    title: 'Chinese — Ko rule (positional superko)',
    rules: [
      'Positional superko: no move may recreate any whole-board position from earlier in the game — not just the position immediately before the opponent\'s last move.',
      'This resolves edge cases that the basic one-step ko cannot: triple ko, eternal life, approach-move ko, etc.',
      'In normal play the rule is rarely invoked beyond the simple one-step ko pattern; everyday ko-fight intuition transfers directly.',
    ],
    note:
        'Positional superko is what makes Chinese rules "fully determinate" — every cyclic position has a defined legality without no-result rulings.',
  ),
  _RuleSection(
    title: 'Chinese — suicide & pass procedure',
    rules: [
      'Suicide is forbidden. You may not place a stone that would result in your own stone or group having zero liberties unless the move simultaneously captures opponent stones (which would restore at least one liberty).',
      'Pass stones: when a player passes, they conventionally place a stone of their own colour on any empty point in their own territory (or give it to the opponent to place). This procedure makes Chinese area scoring give the same answer as Japanese territory scoring on the same final board.',
      'A pass under Chinese rules costs the passer nothing: the pass-stone is placed in your own territory and the stone-count gain offsets the lost empty point.',
    ],
  ),
  _RuleSection(
    title: 'Chinese — life/death conventions',
    rules: [
      'Bent four in the corner: NOT automatically dead. It must be resolved by play; the player with the bent-four group may fight a Ko to live.',
      'Seki: shared empty intersections between mutually-live groups do not count as territory for either player.',
      'Disputed groups are resolved by resuming play. Because passes cost nothing, resumption is free of scoring side-effects.',
      'Used in: most of mainland East Asia outside Japan and Korea, and increasingly on international online servers because life/death disputes always resolve to "play it out."',
    ],
  ),
];

const _japaneseSections = <_RuleSection>[
  _RuleSection(
    title: 'Japanese — komi & scoring',
    rules: [
      'Komi: 6.5 points in modern professional play (raised from 5.5 in 2002 by the Nihon Ki-in).',
      'Territory + prisoners scoring: each player\'s score = (empty intersections surrounded by their stones only) + (number of opponent stones they captured during the game).',
      'Stones on the board do NOT contribute to score (only the territory they enclose and the prisoners they have taken).',
      'Dame: filled in by either player at the end before counting, but dame fillings add nothing to either score.',
      'Practical counting: prisoners are placed into the opponent\'s territory to "fill in" before counting the remaining empty points.',
    ],
    note:
        'The Japanese ruleset is the historical standard for most Western and online Go. More elegant for human counting, but generates more rules-level edge cases than area-scoring rulesets.',
  ),
  _RuleSection(
    title: 'Japanese — Ko rule (basic ko only)',
    rules: [
      'Only the basic ko rule applies — only the immediately previous whole-board position is forbidden.',
      'There is no general superko rule. Exotic repeating positions (triple ko, eternal life, sending-two-returning-one) resolve to "no result" by convention, and the game is replayed.',
      'Long-cycle judgment is by convention rather than a positional rule, so unusual rulings can be tournament-dependent.',
    ],
  ),
  _RuleSection(
    title: 'Japanese — suicide & pass procedure',
    rules: [
      'Suicide forbidden, same as most rulesets. A move giving your own stone or group zero liberties with no capture is illegal.',
      'Regular pass: no pass-stone, no prisoner exchange. A pass simply transfers the turn.',
      'During resumed play after a life/death dispute, a pass may transfer a point to the opponent — this is the "pass cost" that Chinese and AGA rules avoid. Relevant mainly in formal tournament play.',
    ],
  ),
  _RuleSection(
    title: 'Japanese — life/death conventions',
    rules: [
      'Bent four in the corner is PRESUMED DEAD by ruling — the famous Japanese exception. The bent-four group is dead even if it could theoretically fight a ko, because the side with the group cannot prove life without ko threats.',
      'Seki: shared empty intersections between mutually-live groups do not count for either player. Eyes inside a seki group also do not count as territory (an exception to normal territory counting).',
      'Disputed groups may be resolved by analysis or referee ruling, not always by playing them out.',
      'Used in: Japan (Nihon Ki-in 1989 ruleset is the formal reference), most Western tournaments historically, and traditional online Go servers.',
    ],
  ),
];

const _koreanSections = <_RuleSection>[
  _RuleSection(
    title: 'Korean — komi & scoring',
    rules: [
      'Komi: 6.5 points (same as Japanese).',
      'Territory + prisoners scoring — identical in structure to Japanese rules. Score = surrounded empty intersections + prisoners taken.',
      'Stones on the board do not contribute to score (only territory and prisoners).',
      'Practical counting follows the Japanese procedure: prisoners fill the opponent\'s territory, then remaining empty points are counted.',
    ],
    note:
        'Used by the Korean Baduk Association. Functionally near-identical to Japanese rules; the differences are tournament-procedural rather than gameplay-level.',
  ),
  _RuleSection(
    title: 'Korean — Ko rule (basic ko only)',
    rules: [
      'Basic ko only — only the immediately previous board position is forbidden. Same as Japanese rules.',
      'Long-cycle positions (triple ko, eternal life) resolve to no-result by convention; the game is replayed.',
    ],
  ),
  _RuleSection(
    title: 'Korean — suicide & pass procedure',
    rules: [
      'Suicide forbidden.',
      'Regular pass — no pass-stone exchange.',
    ],
  ),
  _RuleSection(
    title: 'Korean — life/death conventions',
    rules: [
      'Bent four in the corner is presumed dead, following Japanese convention.',
      'Seki handled identically to Japanese rules: shared liberties and seki-internal eyes do not count.',
      'Most rule differences from Japanese show up in tournament procedure: how disputes are arbitrated, time-control conventions, byo-yomi durations. Pure gameplay results are essentially identical to Japanese rules.',
      'Used in: Korean Baduk Association tournaments and Korean online servers.',
    ],
  ),
];

const _agaSections = <_RuleSection>[
  _RuleSection(
    title: 'AGA — komi & scoring',
    rules: [
      'Komi: 7.5 in even games; 0.5 in handicap games.',
      'AGA rules permit EITHER area scoring OR territory + prisoners scoring — the players\' choice. Both methods are guaranteed to produce the same winner thanks to mandatory pass-stones.',
      'Under area scoring: living stones + surrounded empty intersections, like Chinese rules.',
      'Under territory scoring: surrounded empty intersections + prisoners taken, like Japanese rules.',
      'Equivalence between methods is mechanical: with pass-stones enforced, the two scoring methods cannot diverge.',
    ],
    note:
        'AGA rules were designed to be ruleset-neutral and to let players from different traditions play together without surprises. Used in AGA-rated tournaments in North America.',
  ),
  _RuleSection(
    title: 'AGA — Ko rule (situational superko)',
    rules: [
      'Situational superko: no move may recreate a prior whole-board position WITH THE SAME PLAYER TO MOVE.',
      'Slightly weaker than Chinese positional superko (which forbids any prior position regardless of whose turn it is).',
      'Triple ko and similar resolve via superko rather than by no-result convention.',
    ],
  ),
  _RuleSection(
    title: 'AGA — suicide & pass procedure',
    rules: [
      'Suicide forbidden.',
      'Mandatory pass-stones: every pass requires the passing player to give one of their stones to the opponent as a prisoner. This is the mechanism that keeps area and territory scoring equivalent.',
      'Two consecutive passes still end the game; the pass-stones simply balance the turn count between the two scoring methods.',
    ],
  ),
  _RuleSection(
    title: 'AGA — life/death conventions',
    rules: [
      'No presumption rules. Disputed groups must be resolved by playing them out, like Chinese rules.',
      'Bent four in the corner is NOT presumed dead — it must be played out.',
      'Seki: shared liberties do not count for either player; treatment of seki-internal eyes follows the chosen scoring method.',
      'Used in: AGA tournaments, the U.S. Open, and AGA-rated online play.',
    ],
  ),
];

const _ingSections = <_RuleSection>[
  _RuleSection(
    title: 'Ing — komi & scoring',
    rules: [
      'Komi: 8 points (full integer, not half). Ties are resolved by "Black wins" — the half-point tie-breaker is replaced by a convention.',
      'Area scoring with fill-in: players fill all dame before counting, and each player is issued a fixed number of stones (180) which they must use.',
      'The fill-in procedure removes any ambiguity about neutral points; what remains is a strict stones-on-board count plus enclosed empty intersections.',
      'Counting reduces to a simple arithmetic check once fill-in is complete.',
    ],
    note:
        'Created by Taiwanese industrialist Ing Chang-ki and used in the Ing Cup (one of the major international Go titles). The most complex commercial ruleset, designed to eliminate "no result" outcomes.',
  ),
  _RuleSection(
    title: 'Ing — Ko rule (Ing superko)',
    rules: [
      'Ing superko: a sophisticated situational superko with explicit handling for "disturbing-ko" situations.',
      'Simple ko is prohibited as basic ko. Multi-step cycles are governed by a specific cycle-detection rule rather than blanket positional repetition.',
      'Designed so that almost every position has a determinate result without invoking no-result rulings — the central design goal of the Ing rules.',
    ],
  ),
  _RuleSection(
    title: 'Ing — suicide & pass procedure',
    rules: [
      'Suicide is ALLOWED. Ing rules permit self-capture moves; the suicided group is removed immediately as if captured by the opponent. (Unique among major rulesets, along with New Zealand.)',
      'Pass procedure: a pass-stone is placed into the opponent\'s bowl (similar to AGA\'s pass mechanic). Required.',
    ],
  ),
  _RuleSection(
    title: 'Ing — life/death conventions',
    rules: [
      'Life/death is resolved by play; no presumptions. Bent four in the corner is resolved by ko fight, not by ruling.',
      'The detailed Ing rules text is the longest of any major ruleset, with explicit handling for edge cases other rulesets resolve by convention.',
      'Used in: the Ing Cup and occasionally in Taiwan. Rare outside those settings due to complexity.',
    ],
  ),
];

const _nzSections = <_RuleSection>[
  _RuleSection(
    title: 'New Zealand — komi & scoring',
    rules: [
      'Komi: 7 (a whole number — ties are possible). By convention White wins ties, or the game is recorded as a tie.',
      'Area scoring — living stones + surrounded empty intersections, identical in structure to Chinese scoring.',
      'No pass-stones; equivalence with territory scoring is not enforced (and not needed, since NZ uses area scoring exclusively).',
    ],
    note:
        'Used by the New Zealand Go Society. Often cited as the simplest national ruleset because it has the fewest special-case rules.',
  ),
  _RuleSection(
    title: 'New Zealand — Ko rule (situational superko)',
    rules: [
      'Situational superko: no move may recreate a prior whole-board position with the same player to move.',
      'Functionally similar to AGA superko; shares the "no no-result" property.',
    ],
  ),
  _RuleSection(
    title: 'New Zealand — suicide & pass procedure',
    rules: [
      'Suicide is ALLOWED. A move that would give your own stone or group zero liberties is legal — the suicided stones are simply removed from the board.',
      'Because NZ uses area scoring, the captured stones don\'t enter a prisoner count — suicide is purely a board-state change.',
      'Pass: regular pass, no exchange. Two consecutive passes end the game.',
    ],
  ),
  _RuleSection(
    title: 'New Zealand — life/death conventions',
    rules: [
      'No life/death presumptions. Disputed groups are played out.',
      'Seki: shared empty points do not count as territory for either player.',
      'Used in: NZ Go Society tournaments. The ruleset is sometimes recommended in academic treatments because of its minimal rule count.',
    ],
  ),
];

const _trompTaylorSections = <_RuleSection>[
  _RuleSection(
    title: 'Tromp–Taylor — komi & scoring',
    rules: [
      'Komi: configurable. Commonly 7.5 in computer Go and engine matchups; Tromp–Taylor itself defines no specific komi value.',
      'Area scoring, defined precisely as: a player\'s score = (stones of their colour on the board) + (empty intersections reachable, via empty paths, ONLY through that player\'s colour).',
      'That precise definition is fully algorithmic — no agreement step between players is needed to compute the score.',
    ],
    note:
        'Defined by John Tromp and Bill Taylor as the minimal unambiguous rules of Go. Used as the engine ruleset for KataGo, AlphaGo, and effectively all modern Go AIs because it requires no human judgment.',
  ),
  _RuleSection(
    title: 'Tromp–Taylor — Ko rule (positional superko)',
    rules: [
      'Positional superko: no move may recreate any prior whole-board position. Identical to the Chinese ko rule.',
      'Combined with the strict scoring definition, this makes every position legally determinate.',
    ],
  ),
  _RuleSection(
    title: 'Tromp–Taylor — suicide & pass procedure',
    rules: [
      'Suicide is ALLOWED by the strict Tromp–Taylor definition: a placed stone (and any same-colour group it joins) with zero liberties after the move is simply removed.',
      'Many implementations override this and forbid suicide instead (e.g., KataGo can be configured either way; AGA-style configurations forbid it). Always check engine configuration.',
      'Pass: regular pass. Two consecutive passes end the game.',
    ],
  ),
  _RuleSection(
    title: 'Tromp–Taylor — life/death conventions',
    rules: [
      'There is NO concept of "agreement on dead stones." Practical play must fill in dame and capture dead groups before passing — otherwise those stones count as alive in the final scoring.',
      'This makes Tromp–Taylor unsuited to casual human play (you would have to play out every dead group), but ideal for computer Go where playing it out is cheap.',
      'No presumption rules; no seki special cases beyond the natural consequence of liberty counting and the strict scoring definition.',
      'Used in: virtually all modern Go AI engines, academic Go theory, and computer-versus-computer match play.',
    ],
  ),
];

class RulesScreen extends StatefulWidget {
  final VoidCallback? onBack;
  const RulesScreen({super.key, this.onBack});

  @override
  State<RulesScreen> createState() => _RulesScreenState();
}

class _RulesScreenState extends State<RulesScreen> {
  Ruleset _selected = Ruleset.chinese;

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    final text = Theme.of(context).textTheme;
    final variation = _variationSections[_selected]!;
    final commonCount = _commonSections.length;
    return Column(
      children: [
        Padding(
          padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 8),
          child: Row(
            children: [
              if (widget.onBack != null)
                IconButton(
                    onPressed: widget.onBack,
                    icon: const Icon(Icons.arrow_back)),
              Text('Rules',
                  style: text.headlineSmall
                      ?.copyWith(fontWeight: FontWeight.w600)),
            ],
          ),
        ),
        Expanded(
          child: SingleChildScrollView(
            padding: const EdgeInsets.symmetric(horizontal: 20, vertical: 12),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: [
                _SectionHeader(label: 'Common rules', scheme: scheme),
                const SizedBox(height: 8),
                for (var i = 0; i < commonCount; i++) ...[
                  _RuleSectionCard(
                      index: i + 1,
                      section: _commonSections[i],
                      scheme: scheme),
                  const SizedBox(height: 12),
                ],
                _VariationPickerCard(
                  selected: _selected,
                  onChanged: (r) => setState(() => _selected = r),
                  scheme: scheme,
                ),
                const SizedBox(height: 12),
                for (var i = 0; i < variation.length; i++) ...[
                  _RuleSectionCard(
                      index: commonCount + i + 1,
                      section: variation[i],
                      scheme: scheme),
                  if (i != variation.length - 1) const SizedBox(height: 12),
                ],
              ],
            ),
          ),
        ),
      ],
    );
  }
}

class _SectionHeader extends StatelessWidget {
  final String label;
  final ColorScheme scheme;
  const _SectionHeader({required this.label, required this.scheme});

  @override
  Widget build(BuildContext context) {
    final text = Theme.of(context).textTheme;
    return Padding(
      padding: const EdgeInsets.only(left: 4, top: 4, bottom: 4),
      child: Text(
        label.toUpperCase(),
        style: text.labelMedium?.copyWith(
          color: scheme.onSurfaceVariant,
          letterSpacing: 1.2,
          fontWeight: FontWeight.w600,
        ),
      ),
    );
  }
}

class _VariationPickerCard extends StatelessWidget {
  final Ruleset selected;
  final ValueChanged<Ruleset> onChanged;
  final ColorScheme scheme;
  const _VariationPickerCard({
    required this.selected,
    required this.onChanged,
    required this.scheme,
  });

  @override
  Widget build(BuildContext context) {
    final text = Theme.of(context).textTheme;
    return ZenCard(
      container: scheme.surfaceContainerLow,
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          _SectionHeader(label: 'Variation rules', scheme: scheme),
          const SizedBox(height: 4),
          Text(
            'Pick a ruleset to see how it differs from the common rules above.',
            style: text.bodyMedium?.copyWith(color: scheme.onSurfaceVariant),
          ),
          const SizedBox(height: 12),
          SizedBox(
            height: 44,
            child: ListView.separated(
              scrollDirection: Axis.horizontal,
              itemCount: Ruleset.values.length,
              separatorBuilder: (_, __) => const SizedBox(width: 8),
              itemBuilder: (context, i) {
                final r = Ruleset.values[i];
                return SizedBox(
                  width: r == Ruleset.newZealand || r == Ruleset.trompTaylor
                      ? 130
                      : 100,
                  child: ZenOptionButton(
                    label: r.label,
                    selected: r == selected,
                    onTap: () => onChanged(r),
                  ),
                );
              },
            ),
          ),
          const SizedBox(height: 12),
          Container(
            width: double.infinity,
            padding: const EdgeInsets.all(12),
            decoration: BoxDecoration(
              color: scheme.surfaceContainerHigh,
              borderRadius: BorderRadius.circular(8),
            ),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  selected.label,
                  style: text.titleMedium
                      ?.copyWith(fontWeight: FontWeight.w600),
                ),
                const SizedBox(height: 4),
                Text(
                  selected.tagline,
                  style: text.bodySmall
                      ?.copyWith(color: scheme.onSurfaceVariant),
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }
}

class _RuleSectionCard extends StatelessWidget {
  final int index;
  final _RuleSection section;
  final ColorScheme scheme;
  const _RuleSectionCard(
      {required this.index, required this.section, required this.scheme});

  @override
  Widget build(BuildContext context) {
    final text = Theme.of(context).textTheme;
    return ZenCard(
      container: scheme.surfaceContainerLow,
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Container(
                width: 30,
                height: 30,
                decoration: BoxDecoration(
                  shape: BoxShape.circle,
                  color: scheme.surfaceContainerHigh,
                ),
                alignment: Alignment.center,
                child: Text(
                  index.toString(),
                  style: text.labelMedium
                      ?.copyWith(color: scheme.onSurfaceVariant),
                ),
              ),
              const SizedBox(width: 12),
              Expanded(
                child: Text(section.title,
                    style: text.headlineSmall
                        ?.copyWith(fontWeight: FontWeight.w600)),
              ),
            ],
          ),
          const SizedBox(height: 12),
          ...section.rules.map((rule) => Padding(
                padding: const EdgeInsets.only(bottom: 8),
                child: _RuleParagraph(text: rule, scheme: scheme),
              )),
          if (section.note != null)
            Container(
              width: double.infinity,
              padding: const EdgeInsets.all(12),
              decoration: BoxDecoration(
                color: scheme.surfaceContainerHigh,
                borderRadius: BorderRadius.circular(8),
              ),
              child: Text(section.note!,
                  style: text.bodyMedium
                      ?.copyWith(color: scheme.onSurfaceVariant)),
            ),
        ],
      ),
    );
  }
}

class _RuleParagraph extends StatelessWidget {
  final String text;
  final ColorScheme scheme;
  const _RuleParagraph({required this.text, required this.scheme});

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context).textTheme;
    final colon = text.indexOf(':');
    final hasLead = colon >= 1 && colon <= 42;
    return Row(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Padding(
          padding: const EdgeInsets.only(top: 8, right: 10),
          child: Container(
            width: 6,
            height: 6,
            decoration: BoxDecoration(
              shape: BoxShape.circle,
              color: scheme.onSurfaceVariant.withValues(alpha: 0.45),
            ),
          ),
        ),
        Expanded(
          child: hasLead
              ? Text.rich(
                  TextSpan(children: [
                    TextSpan(
                      text: text.substring(0, colon + 1),
                      style: theme.bodyMedium
                          ?.copyWith(fontWeight: FontWeight.w600),
                    ),
                    TextSpan(
                        text: text.substring(colon + 1),
                        style: theme.bodyMedium),
                  ]),
                )
              : Text(text, style: theme.bodyMedium),
        ),
      ],
    );
  }
}
