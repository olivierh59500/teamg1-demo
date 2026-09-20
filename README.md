# TEAMG1 Demo

Hommage moderne à la demoscene, écrit en Go avec Ebitengine par Bilizir de DMA.

La démo combine un shader CRT, un plasma temps réel, un cube 3D texturé, des
logos déformés, un scroller sinusoïdal et une musique YM2149.

## Prérequis

- Go 1.25 ou plus récent ;
- pour Android : JDK 17, SDK Android 36, Build Tools 36 et NDK 28.2.13676358.

Les versions applicatives sont verrouillées dans `go.mod` : Ebitengine 2.9.11
et `ym-player` à la révision validée du 13 septembre 2026. L’audio est synthétisé
et joué à 48 kHz.

## Version ordinateur

```sh
go run ./cmd/teamg1demo
```

Pour construire un exécutable :

```sh
go build -o teamg1-demo ./cmd/teamg1demo
```

La touche `F` active ou désactive le plein écran.

## Tests

```sh
go test ./...
go test -race ./...
go vet ./...
golangci-lint run ./...
```

Les tests couvrent notamment le PCM stéréo, l’absence d’allocation dans la
lecture YM, le rendu du plasma, le layout large et la garde de rendu desktop.

## Version Android

Avec un unique appareil Android ARM64 connecté, déverrouillé et autorisé :

```sh
./scripts/run-android.sh
```

Le script génère l’AAR avec la même version d’Ebitengine que `go.mod`, compile
l’APK, l’installe puis lance `com.olivierh.teamg1demo/.MainActivity`.

Pour construire sans installer :

```sh
mkdir -p android/app/libs
go run github.com/hajimehoshi/ebiten/v2/cmd/ebitenmobile@v2.9.11 \
  bind \
  -target android/arm64 \
  -androidapi 23 \
  -javapkg com.olivierh.teamg1demo \
  -o android/app/libs/teamg1demo.aar \
  ./mobile

./android/gradlew -p android --console=plain clean assembleDebug
./android/gradlew -p android --console=plain lintDebug
```

L’APK de débogage est produit dans
`android/app/build/outputs/apk/debug/app-debug.apk`.

## Optional DCK version

The original implementation remains at its original paths. Run it with `go run ./cmd/teamg1demo`.

The construction-kit version is in [dck/](dck/README.md). Run `go run ./dck/cmd/teamg1demo` from this directory. Both versions share the original assets.
