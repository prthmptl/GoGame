import '../game_state.dart';
import '../models.dart';

abstract class GoAi {
  MoveIntent chooseMove(GameState state);
}
