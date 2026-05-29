import 'package:flutter/foundation.dart';
import 'package:shared_preferences/shared_preferences.dart';

const _kName = 'profile.name';
const _kCountry = 'profile.country';
const _kRating = 'profile.rating';
const _kOnboarded = 'profile.onboarded';

class ProfileData {
  final String name;
  final String countryEmoji;
  final int rating;
  final bool onboarded;

  const ProfileData({
    required this.name,
    required this.countryEmoji,
    required this.rating,
    required this.onboarded,
  });

  ProfileData copyWith({
    String? name,
    String? countryEmoji,
    int? rating,
    bool? onboarded,
  }) =>
      ProfileData(
        name: name ?? this.name,
        countryEmoji: countryEmoji ?? this.countryEmoji,
        rating: rating ?? this.rating,
        onboarded: onboarded ?? this.onboarded,
      );
}

class ProfileStore extends ChangeNotifier {
  final SharedPreferences _prefs;
  ProfileData _value;

  ProfileStore._(this._prefs, this._value);

  static Future<ProfileStore> load() async {
    final prefs = await SharedPreferences.getInstance();
    return ProfileStore._(
      prefs,
      ProfileData(
        name: prefs.getString(_kName) ?? 'Player',
        countryEmoji: prefs.getString(_kCountry) ?? '',
        rating: prefs.getInt(_kRating) ?? 1000,
        onboarded: prefs.getBool(_kOnboarded) ?? false,
      ),
    );
  }

  ProfileData get value => _value;

  Future<void> update(ProfileData Function(ProfileData) transform) async {
    final next = transform(_value);
    await _prefs.setString(_kName, next.name);
    await _prefs.setString(_kCountry, next.countryEmoji);
    await _prefs.setInt(_kRating, next.rating);
    await _prefs.setBool(_kOnboarded, next.onboarded);
    _value = next;
    notifyListeners();
  }
}
