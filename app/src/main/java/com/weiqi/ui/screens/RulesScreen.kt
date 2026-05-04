package com.weiqi.ui.screens

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.ArrowBack
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.text.SpanStyle
import androidx.compose.ui.text.buildAnnotatedString
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.withStyle
import androidx.compose.ui.unit.dp
import com.weiqi.ui.components.ZenCard

private data class RuleSection(
    val title: String,
    val rules: List<String>,
    val note: String? = null
)

@Composable
fun RulesScreen(onBack: () -> Unit) {
    Column(
        modifier = Modifier
            .fillMaxSize()
            .background(MaterialTheme.colorScheme.background)
    ) {
        Row(
            modifier = Modifier
                .fillMaxWidth()
                .padding(horizontal = 8.dp, vertical = 8.dp),
            verticalAlignment = Alignment.CenterVertically
        ) {
            IconButton(onClick = onBack) {
                Icon(Icons.Filled.ArrowBack, contentDescription = "Back")
            }
            Text(
                "Rules",
                style = MaterialTheme.typography.headlineSmall,
                fontWeight = FontWeight.SemiBold
            )
        }

        Column(
            modifier = Modifier
                .fillMaxSize()
                .verticalScroll(rememberScrollState())
                .padding(horizontal = 20.dp, vertical = 12.dp),
            verticalArrangement = Arrangement.spacedBy(12.dp)
        ) {
            ruleSections.forEachIndexed { index, section ->
                RuleSectionCard(index = index + 1, section = section)
            }
        }
    }
}

@Composable
private fun RuleSectionCard(index: Int, section: RuleSection) {
    ZenCard(
        modifier = Modifier.fillMaxWidth(),
        container = MaterialTheme.colorScheme.surfaceContainerLow
    ) {
        Column(verticalArrangement = Arrangement.spacedBy(12.dp)) {
            Row(verticalAlignment = Alignment.CenterVertically) {
                Box(
                    modifier = Modifier
                        .size(30.dp)
                        .clip(CircleShape)
                        .background(MaterialTheme.colorScheme.surfaceContainerHigh),
                    contentAlignment = Alignment.Center
                ) {
                    Text(
                        index.toString(),
                        style = MaterialTheme.typography.labelMedium,
                        color = MaterialTheme.colorScheme.onSurfaceVariant
                    )
                }
                Text(
                    section.title,
                    modifier = Modifier.padding(start = 12.dp),
                    style = MaterialTheme.typography.headlineSmall,
                    fontWeight = FontWeight.SemiBold
                )
            }

            section.rules.forEach { rule ->
                RuleParagraph(rule)
            }

            section.note?.let { note ->
                Text(
                    note,
                    modifier = Modifier
                        .fillMaxWidth()
                        .background(
                            MaterialTheme.colorScheme.surfaceContainerHigh,
                            RoundedCornerShape(8.dp)
                        )
                        .padding(12.dp),
                    style = MaterialTheme.typography.bodyMedium,
                    color = MaterialTheme.colorScheme.onSurfaceVariant
                )
            }
        }
    }
}

@Composable
private fun RuleParagraph(text: String) {
    Row(
        modifier = Modifier.fillMaxWidth(),
        horizontalArrangement = Arrangement.spacedBy(10.dp)
    ) {
        Box(
            modifier = Modifier
                .padding(top = 8.dp)
                .size(6.dp)
                .clip(CircleShape)
                .background(MaterialTheme.colorScheme.onSurfaceVariant.copy(alpha = 0.45f))
        )
        Text(
            text = ruleText(text),
            modifier = Modifier.weight(1f),
            style = MaterialTheme.typography.bodyMedium,
            color = MaterialTheme.colorScheme.onSurface
        )
    }
}

private fun ruleText(text: String) = buildAnnotatedString {
    val colon = text.indexOf(':')
    if (colon in 1..42) {
        withStyle(SpanStyle(fontWeight = FontWeight.SemiBold)) {
            append(text.substring(0, colon + 1))
        }
        append(text.substring(colon + 1))
    } else {
        append(text)
    }
}

private val ruleSections = listOf(
    RuleSection(
        title = "Equipment & setup",
        rules = listOf(
            "Board: Standard play uses a 19×19 grid. 13×13 and 9×9 boards are used for teaching and faster games. The lines form 361 intersections on a 19×19 board.",
            "Stones: One player uses black stones, the other uses white stones. Stones are flat and lens-shaped. Each set conventionally contains 181 black and 180 white stones (enough to fill the board).",
            "Star points (hoshi): Nine intersections on a 19x19 board are marked with dots. These serve as visual reference points and as handicap stone placement positions.",
            "Komi: Because Black moves first and has a first-move advantage, White receives a points bonus called komi. Under Chinese rules, komi is 7.5 points. The 0.5 fraction eliminates the possibility of a tie."
        )
    ),
    RuleSection(
        title = "Objective of the game",
        rules = listOf(
            "The goal is to control more territory than your opponent. Territory is defined as empty intersections completely surrounded by one player's stones.",
            "Under Chinese rules specifically, the final score equals the number of live stones on the board plus the number of empty intersections those stones surround. Captured stones are not directly deducted from score (unlike Japanese rules).",
            "The player with the higher score wins. With komi, a score difference of even 0.5 points decides the game. Ties are impossible."
        )
    ),
    RuleSection(
        title = "Placing stones",
        rules = listOf(
            "Black moves first. Players alternate turns. On your turn you must either place one stone on any unoccupied intersection, or pass.",
            "Stones are placed on intersections (the crossing points of lines), not inside squares.",
            "Once placed, stones do not move. They remain on their intersection unless captured.",
            "You may play on any empty intersection subject to the two constraints: the Ko rule and the Suicide rule (see below).",
            "Passing: A player may pass their turn at any time. Passing does not forfeit the game. However, a stone passed under Chinese rules is simply placed in your own territory. Captured stones are not exchanged as bowl stones."
        )
    ),
    RuleSection(
        title = "Liberties & capture",
        rules = listOf(
            "Liberties are the empty intersections directly adjacent (up, down, left, right, never diagonal) to a stone or a connected group of stones.",
            "When all liberties of a stone or group are occupied by the opponent's stones, that stone or group is captured and immediately removed from the board.",
            "The capturing player removes the captured stones at the moment of capture (unlike Atari in Shogi. In Go, removal is immediate).",
            "Atari is the state when a stone or group has exactly one liberty remaining. It is in immediate danger of capture. There is no obligation to announce Atari.",
            "Under Chinese rules, captured stones are kept aside but do NOT count against the capturing player's score. The score is purely living stones + controlled empty intersections."
        ),
        note = "Chinese rules differ from Japanese rules here: captured stones do not affect the final score calculation. Only living stones on the board and enclosed empty points count."
    ),
    RuleSection(
        title = "Groups & connectivity",
        rules = listOf(
            "Two stones of the same color that share a common liberty or are connected through a chain of shared liberties form a group (also called a string or chain).",
            "Connectivity is orthogonal only. Diagonally adjacent stones of the same color are NOT part of the same group.",
            "All stones in a group share the same set of liberties. If any liberty of the group is occupied by an enemy stone, every stone in the group loses that liberty.",
            "A group is captured only when all its liberties are occupied. You cannot partially capture a group."
        )
    ),
    RuleSection(
        title = "Suicide rule (self-capture)",
        rules = listOf(
            "You may not place a stone on an intersection that would immediately have zero liberties, unless doing so simultaneously captures one or more of the opponent's stones, which would restore at least one liberty.",
            "Under Chinese rules (and most modern rulesets), self-capture (suicide) is forbidden. A move that would result in your own stone or group having no liberties, without capturing opponent stones, is illegal.",
            "When a stone is placed and captures enemy stones first, the liberties freed by those captures are computed before checking the placed stone's liberty count. If the capturing gives the placed stone at least one liberty, the move is legal."
        ),
        note = "Suicide is illegal under Chinese rules. Always check that your placed stone (or the group it joins) will have at least one liberty after any captures resolve."
    ),
    RuleSection(
        title = "The Ko rule",
        rules = listOf(
            "Ko (劫) prevents an infinite loop. The rule states: A player may not make a move that returns the board to the exact same position as it was immediately before the opponent's last move.",
            "This most commonly occurs in a 'Ko fight,' a specific one-stone pattern where each player could repeatedly recapture the other's stone, cycling indefinitely without Ko.",
            "After a Ko capture, the opponent must play elsewhere (a Ko threat) before recapturing. If there is no suitable Ko threat, the Ko can be abandoned.",
            "Under Chinese rules (positional superko): The restriction is broader. No move may recreate any previous board position from the entire game, not just the immediately preceding position. This resolves edge cases in complex multi-Ko situations that basic Ko rules cannot handle.",
            "In practice, the positional superko rule is rarely invoked beyond the simple one-step Ko pattern, but it is the correct rule when resolving unusual cyclic positions like triple Ko, eternal life, or approach-move Ko."
        ),
        note = "Positional superko (no board position may repeat at any point in the game) is the Chinese rule standard. Japanese rules use only the one-step ko rule, which is why Chinese and Japanese rules occasionally give different results in exotic ko situations."
    ),
    RuleSection(
        title = "Life & death",
        rules = listOf(
            "A group is alive if it cannot be captured regardless of how the opponent plays. A group is dead if it cannot avoid eventual capture regardless of how it plays.",
            "Two eyes: The most common way to achieve life is for a group to contain two separate internal empty spaces (eyes) that the opponent cannot simultaneously fill. A group with two genuine eyes cannot be captured.",
            "False eye: An eye-shaped space is false if the opponent can eventually destroy it by playing on the key intersection that connects the surrounding stones. False eyes do not confer life.",
            "Seki (mutual life / impasse): Two opposing groups may share liberties such that neither player can capture the other without first losing their own group. Both groups live without having two eyes. The shared empty intersections between them do not count as territory for either player under Chinese rules.",
            "Ko life: A group that can only maintain life through a Ko fight may be ruled dead or alive depending on the outcome of that Ko. Under Chinese rules, if Ko is fought to completion, the outcome determines life/death.",
            "Bent four in the corner: Under Chinese rules this is NOT automatically dead. It must be resolved by play, unlike Japanese rules where it is presumed dead. The player with the bent-four group may be able to fight a Ko to live."
        )
    ),
    RuleSection(
        title = "End of the game",
        rules = listOf(
            "The game ends when both players pass consecutively. This signals that neither player believes any further moves will increase their score.",
            "Before scoring, players must agree on which stones on the board are dead. Dead stones are removed before counting.",
            "If players disagree on the status (dead or alive) of a group, they resume play to resolve the dispute. The player claiming a group is dead must prove it by capturing those stones in resumed play; the other player defends.",
            "Under Chinese rules, if play resumes to resolve a life/death dispute, any passes during resumed play do not cost points (unlike Japanese rules where passing in this phase can give points to the opponent)."
        )
    ),
    RuleSection(
        title = "Scoring (Chinese area scoring)",
        rules = listOf(
            "Chinese scoring formula: Player's score = (number of their living stones on the board) + (number of empty intersections completely surrounded by their stones).",
            "Captured stones that were removed from the board are not used in scoring. Unlike Japanese territory scoring, Chinese rules count stones still on the board rather than prisoner counts.",
            "White's final score has komi (7.5) added to it. Black wins if Black's score > White's score + 7.5.",
            "Dame (neutral points): Empty intersections that border both colors or that neither player wishes to fill are called dame. Under Chinese rules, dame do not count for either player and are not filled before scoring (though in tournament play, they may be filled to simplify counting).",
            "Equivalence with territory scoring: For a filled board, Chinese area scoring and Japanese territory scoring yield the same winner most of the time. They differ when there are disputes about dame, passes, or bent-four-in-corner situations.",
            "Counting method: In practice, Chinese counting is done by rearranging one player's stones and territory into a rectangular shape for ease of calculation. The total is then compared to 180.5 (half of 361 total points, adjusted for komi)."
        ),
        note = "Quick check: if Black occupies 181+ points (stones + empty surrounded intersections), Black wins before komi adjustment. With komi of 7.5, Black needs 185 points (out of 361) to guarantee a win against White's 176 + 7.5 = 183.5."
    ),
    RuleSection(
        title = "Handicap games",
        rules = listOf(
            "Handicap stones allow players of different strengths to play competitively. The weaker player (Black) places extra stones on the board before White makes any move.",
            "Handicap stones are placed on the star points (hoshi) in a fixed pattern. On a 19×19 board, standard placement positions are: 2 stones = D4 and Q16; 3 stones = add D16; 4 = add Q4; 5 to 9 stones add center and side star points in a traditional order.",
            "In a handicap game, White moves first (since Black has already 'moved' by placing handicap stones).",
            "Komi in handicap games: When stones are given, komi is typically set to 0.5 (instead of 7.5) to avoid doubly compensating White. Some rulesets use komi of 0 with a half-point tie-break rule added separately."
        )
    ),
    RuleSection(
        title = "Rules of conduct & etiquette",
        rules = listOf(
            "Resignation: A player may resign at any time, conceding the game. Resignation is the normal way professional games end. By convention, a player who resigns after a long game should not request to count the score.",
            "Stone placement: Stones must be placed decisively with a clean click. In formal play, once a stone touches the board it must be played at that intersection (no adjusting).",
            "Review after game: It is customary (and required in teaching contexts) to replay the game from memory after it ends to discuss key moments. Both players are expected to remember the game.",
            "Time: In timed games, a player who exceeds their time limit loses immediately. Byo-yomi (overtime) is common. The player must complete each move within a fixed period (e.g., 30 seconds) or lose.",
            "No take-backs: Once placed, a stone cannot be moved or removed by the player who placed it, except as a captured stone by the opponent."
        )
    ),
    RuleSection(
        title = "Chinese rules, specific differences",
        rules = listOf(
            "Area (territory + stones) scoring, not territory-only scoring. Living stones on the board count.",
            "Positional superko: no board position may be repeated at any point in the game (not just the previous move).",
            "Bent four in the corner is not automatically dead. It must be played out.",
            "Pass stones: When a player passes, they place a stone of their own color on any empty point in their own territory (or give one to the opponent to place, depending on variation). This ensures that passing has the same scoring effect as making a neutral move, keeping Chinese area scoring and Japanese territory scoring equivalent.",
            "Suicide is forbidden. Self-capture moves with no offsetting enemy capture are illegal.",
            "Komi is 7.5 in Chinese rules for even games at the professional level. Older Chinese rules used 5.5 komi historically.",
            "No points for captured stones. Prisoners are set aside and do not factor into the score, unlike Japanese rules."
        ),
        note = "The Chinese ruleset is also called 'area scoring' or 'territory + stones scoring.' It is used by China, most of East Asia outside Japan and Korea, and is increasingly common internationally due to its simplicity in resolving life/death disputes."
    )
)
