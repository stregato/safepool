
OUT=$(PWD)/dist
CASPIAN=$(PWD)/../caspian

linux-x86_64:
	CGO_ENABLED=1 \
	GOOS=linux \
	GOARCH=amd64 \
	go build -buildmode=c-shared -tags linux -o $(OUT)/linux/libsafepool.so
linux: linux-x86_64

windows:
	CGO_ENABLED=1 \
	CC=x86_64-w64-mingw32-gcc \
	GOOS=windows \
	GOARCH=amd64 \
	go build -buildmode=c-shared -tags windows -o $(OUT)/windows/safepool.dll

macos-x86_64:
	CGO_ENABLED=1 \
	GOOS=darwin \
	GOARCH=amd64 \
	go build -buildmode=c-shared -tags macos -o $(OUT)/macos/x86/libsafepool.dylib
macos-arm_64:
	CGO_ENABLED=1 \
	GOOS=darwin \
	GOARCH=arm64 \
	go build -buildmode=c-shared -tags macos -o $(OUT)/macos/arm64/libsafepool.dylib
macos: macos-x86_64 macos-arm_64
	lipo $(OUT)/macos/x86/libsafepool.dylib $(OUT)/macos/arm64/libsafepool.dylib -create -output $(OUT)/macos/libsafepool.dylib
	rm -rf $(OUT)/macos/x86 $(OUT)/macos/arm64
	test -e $(CASPIAN) && cp -rf $(OUT)/macos $(CASPIAN)

ios-arm64:
	CGO_ENABLED=1 \
	GOOS=ios \
	GOARCH=arm64 \
	SDK=iphoneos \
	SDK_PATH=`xcrun --sdk iphoneos --show-sdk-path` \
	CC=$(shell go env GOROOT)/misc/ios/clangwrap.sh \
	CGO_CFLAGS="-fembed-bitcode" \
	CLANG=`xcrun --sdk iphoneos --find clang` \
	go build -buildmode=c-archive -tags ios -o $(OUT)/ios/arm64/safepool.a 

ios-x86_64:
	CGO_ENABLED=1 \
	GOOS=ios \
	GOARCH=amd64 \
	SDK=iphonesimulator \
	SDK_PATH=`xcrun --sdk iphonesimulator --show-sdk-path` \
	CC=$(PWD)/clangwrap.sh \
	CLANG=`xcrun --sdk iphonesimulator --find clang` \
	go build -buildmode=c-archive -tags ios -o $(OUT)/ios/x86/safepool.a

ios: ios-arm64 ios-x86_64
#	lipo $(OUT)/ios/arm64/libsafepool.a $(OUT)/ios/x86/libsafepool.a -create -output $(OUT)/ios/libsafepool.a
	rm -rf $(OUT)/ios/Safepool.xcframework
	xcodebuild -create-xcframework \
    -output $(OUT)/ios/Safepool.xcframework \
    -library $(OUT)/ios/arm64/safepool.a \
    -headers $(OUT)/ios/arm64/safepool.h \
    -library $(OUT)/ios/x86/safepool.a  \
    -headers $(OUT)/ios/x86/safepool.h
	rm -r $(OUT)/ios/arm64
	rm -r $(OUT)/ios/x86
	test -e $(CASPIAN) && cp -rf $(OUT)/ios $(CASPIAN)

apple: macos ios

ANDROID_OUT=../caspian/android/app/src/main/jniLibs
ANDROID_SDK=$(HOME)/Android/Sdk
NDK_VERSION=25.1.8937393
NDK_BUILD=linux-x86_64
NDK_BIN=$(ANDROID_SDK)/ndk/$(NDK_VERSION)/toolchains/llvm/prebuilt/$(NDK_BUILD)/bin
# android-armv7a:
# 	CGO_ENABLED=1 \
# 	GOOS=android \
# 	GOARCH=arm \
# 	GOARM=7 \
# 	CC=$(NDK_BIN)/armv7a-linux-androideabi21-clang \
# 	go build -v -buildmode=c-shared -o $(ANDROID_OUT)/armeabi-v7a/libsafepool.so

android-arm64:
	CGO_ENABLED=1 \
	GOOS=android \
	GOARCH=arm64 \
	CC=$(NDK_BIN)/aarch64-linux-android21-clang \
	go build -v -buildmode=c-shared -o $(OUT)/android/arm64-v8a/libsafepool.so

# android-x86:
# 	CGO_ENABLED=1 \
# 	GOOS=android \
# 	GOARCH=386 \
# 	CC=$(NDK_BIN)/i686-linux-android21-clang \
# 	go build -v -buildmode=c-shared -o $(ANDROID_OUT)/x86/libsafepool.so

android-x86_64:
	CGO_ENABLED=1 \
	GOOS=android \
	GOARCH=amd64 \
	CC=$(NDK_BIN)/x86_64-linux-android21-clang \
	go build -v -buildmode=c-shared -o $(OUT)/android/x86_64/libsafepool.so

#android: android-armv7a android-arm64 android-x86 android-x86_64
android: android-arm64 android-x86_64

SNAP_OUT=$(CASPIAN)/snap/local
snap:
	go mod tidy
	CGO_ENABLED=1 \
	GOOS=linux \
	GOARCH=amd64 \
	go build -buildmode=c-shared -tags linux -o $(PREFIX)/libsafepool.so

caspian:
	test -e $(CASPIAN) && cp -rf $(OUT)/* $(CASPIAN)

	