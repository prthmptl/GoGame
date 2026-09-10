/// Minimal SGF tree parser. Builds a node tree preserving variations so
/// callers can walk both the main line and sidelines.
///
/// We don't aim for full SGF coverage — just enough to handle FF[4] game
/// records that the rest of the app exports/consumes.
class SgfTreeNode {
  /// Properties present at this node, e.g. {`B` → [`pd`], `C` → [`hello`]}.
  final Map<String, List<String>> properties;

  /// Variations rooted at this node. The first entry is the main line.
  final List<SgfTreeNode> children;

  SgfTreeNode({
    Map<String, List<String>>? properties,
    List<SgfTreeNode>? children,
  })  : properties = properties ?? <String, List<String>>{},
        children = children ?? <SgfTreeNode>[];

  bool get isEmpty => properties.isEmpty && children.isEmpty;

  /// Convenience accessor for the first value of a property, or `null`.
  String? prop(String key) {
    final list = properties[key];
    if (list == null || list.isEmpty) return null;
    return list.first;
  }
}

class SgfTreeParseException implements Exception {
  final String message;
  final int offset;
  SgfTreeParseException(this.message, this.offset);
  @override
  String toString() => 'SgfTreeParseException at $offset: $message';
}

class SgfTreeParser {
  static SgfTreeNode parse(String input) {
    if (input.length > 2 * 1024 * 1024) {
      throw SgfTreeParseException('SGF exceeds 2 MB', 0);
    }
    final parser = SgfTreeParser._(input);
    parser._skipWhitespace();
    if (parser._peek() != '(') {
      throw SgfTreeParseException('expected "("', parser._pos);
    }
    final root = parser._parseSequence();
    parser._skipWhitespace();
    if (parser._pos != input.length) {
      throw SgfTreeParseException('Unexpected trailing data', parser._pos);
    }
    return root;
  }

  final String _src;
  int _pos = 0;
  int _depth = 0;

  SgfTreeParser._(this._src);

  /// Parse `( ;n1 ;n2 ... (sub) (sub) )` returning the head node with the
  /// trailing nodes chained as the first child each.
  SgfTreeNode _parseSequence() {
    if (++_depth > 128) {
      throw SgfTreeParseException('SGF variations nested too deeply', _pos);
    }
    _expect('(');
    final nodes = <SgfTreeNode>[];
    final variations = <SgfTreeNode>[];
    while (true) {
      _skipWhitespace();
      final ch = _peek();
      if (ch == null) {
        throw SgfTreeParseException('unexpected end of input', _pos);
      }
      if (ch == ';') {
        _pos++;
        nodes.add(_parseProperties());
      } else if (ch == '(') {
        variations.add(_parseSequence());
      } else if (ch == ')') {
        _pos++;
        break;
      } else {
        throw SgfTreeParseException('unexpected character "$ch"', _pos);
      }
    }
    _depth--;
    // Stitch: nodes[0] → nodes[1] → … → nodes[n].children = variations
    if (nodes.isEmpty) throw SgfTreeParseException('Empty SGF game tree', _pos);
    for (var i = 0; i < nodes.length - 1; i++) {
      nodes[i].children.add(nodes[i + 1]);
    }
    nodes.last.children.addAll(variations);
    return nodes.first;
  }

  SgfTreeNode _parseProperties() {
    final node = SgfTreeNode();
    while (true) {
      _skipWhitespace();
      final ch = _peek();
      if (ch == null) break;
      if (ch == ';' || ch == '(' || ch == ')') break;
      final key = _parseIdent();
      if (key.isEmpty) {
        throw SgfTreeParseException('expected property identifier', _pos);
      }
      final values = <String>[];
      _skipWhitespace();
      while (_peek() == '[') {
        values.add(_parseValue());
        _skipWhitespace();
      }
      if (values.isEmpty) {
        throw SgfTreeParseException('Missing property value', _pos);
      }
      node.properties.putIfAbsent(key, () => <String>[]).addAll(values);
    }
    return node;
  }

  String _parseIdent() {
    final start = _pos;
    while (_pos < _src.length) {
      final c = _src.codeUnitAt(_pos);
      // SGF identifiers are upper-case letters.
      if (c >= 0x41 && c <= 0x5A) {
        _pos++;
      } else {
        break;
      }
    }
    return _src.substring(start, _pos);
  }

  String _parseValue() {
    _expect('[');
    final buf = StringBuffer();
    while (_pos < _src.length) {
      final c = _src[_pos];
      if (c == '\\' && _pos + 1 < _src.length) {
        // SGF escapes: backslash + char keeps the char (with newline removed).
        final next = _src[_pos + 1];
        if (next != '\n' && next != '\r') buf.write(next);
        _pos += 2;
        if (next == '\r' && _peek() == '\n') _pos++;
        continue;
      }
      if (c == ']') {
        _pos++;
        return buf.toString();
      }
      buf.write(c);
      _pos++;
    }
    throw SgfTreeParseException('unterminated property value', _pos);
  }

  void _skipWhitespace() {
    while (_pos < _src.length) {
      final c = _src.codeUnitAt(_pos);
      // Spaces, tabs, newlines.
      if (c == 0x20 || c == 0x09 || c == 0x0A || c == 0x0D) {
        _pos++;
      } else {
        break;
      }
    }
  }

  String? _peek() => _pos < _src.length ? _src[_pos] : null;

  void _expect(String c) {
    if (_pos >= _src.length || _src[_pos] != c) {
      throw SgfTreeParseException('expected "$c"', _pos);
    }
    _pos++;
  }
}
