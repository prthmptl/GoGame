import 'dart:convert';

import 'package:flutter/foundation.dart';
import 'package:flutter/services.dart' show rootBundle;
import 'package:shared_preferences/shared_preferences.dart';

import '../domain/lessons/course.dart';

const _kCompletedPrefix = 'lesson.done.';

/// Loads authored courses from JSON assets and tracks per-lesson completion.
class LessonRepo extends ChangeNotifier {
  static const _assetPath = 'assets/lessons/courses.json';
  final SharedPreferences _prefs;
  List<Course> _courses = const [];

  LessonRepo._(this._prefs);

  static Future<LessonRepo> load() async {
    final prefs = await SharedPreferences.getInstance();
    final repo = LessonRepo._(prefs);
    await repo._loadAll();
    return repo;
  }

  Future<void> _loadAll() async {
    final raw = await rootBundle.loadString(_assetPath);
    _courses = (json.decode(raw) as List<dynamic>)
        .map((e) => Course.fromJson(e as Map<String, dynamic>))
        .toList(growable: false);
  }

  List<Course> get courses => _courses;

  Course? byId(String id) {
    for (final c in _courses) {
      if (c.id == id) return c;
    }
    return null;
  }

  bool isLessonComplete(String courseId, String lessonId) =>
      _prefs.getBool('$_kCompletedPrefix$courseId.$lessonId') ?? false;

  int completedCount(Course course) =>
      course.lessons.where((l) => isLessonComplete(course.id, l.id)).length;

  Future<void> markComplete(String courseId, String lessonId) async {
    await _prefs.setBool('$_kCompletedPrefix$courseId.$lessonId', true);
    notifyListeners();
  }
}
