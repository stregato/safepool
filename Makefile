
LINUX_OUT=../caspian/linux/libs
SNAP_OUT=../caspian/snap/local
linux-x86_64:
	CGO_ENABLED=1 \
	GOOS=linux \
	GOARCH=amd64 \
	go build -buildmode=c-shared -tags linux -o $(LINUX_OUT)/amd64/libsafepool.so

snap:
	go mod tidy
	CGO_ENABLED=1 \
	GOOS=linux \
	GOARCH=amd64 \
	go build -buildmode=c-shared -tags linux -o $(PREFIX)/libsafepool.so

MAC_OUT=../caspian/macos/libs
macos-x86_64:
	CGO_ENABLED=1 \
	GOOS=darwin \
	GOARCH=amd64 \
	go build -buildmode=c-shared -tags macos -o $(MAC_OUT)/amd64/libsafepool.dylib
macos-arm_64:
	CGO_ENABLED=1 \
	GOOS=darwin \
	GOARCH=arm64 \
	go build -buildmode=c-shared -tags macos -o $(MAC_OUT)/arm64/libsafepool.dylib
macos: macos-x86_64 macos-arm_64
	lipo $(MAC_OUT)/amd64/libsafepool.dylib $(MAC_OUT)/arm64/libsafepool.dylib -create -output $(MAC_OUT)/libsafepool.dylib

IOS_OUT=../caspian/ios
ios-arm64:
	CGO_ENABLED=1 \
	GOOS=darwin \
	GOARCH=arm64 \
	SDK=iphoneos \
	CC=$(shell go env GOROOT)/misc/ios/clangwrap.sh \
	CGO_CFLAGS="-fembed-bitcode" \
	go build -buildmode=c-archive -tags ios -o $(IOS_OUT)/arm64.a 

ios-x86_64:
	CGO_ENABLED=1 \
	GOOS=darwin \
	GOARCH=amd64 \
	SDK=iphonesimulator \
	CC=$(PWD)/clangwrap.sh \
	go build -buildmode=c-archive -tags ios -o $(IOS_OUT)/x86_64.a

ios: ios-arm64 ios-x86_64
	lipo $(IOS_OUT)/x86_64.a $(IOS_OUT)/arm64.a -create -output $(IOS_OUT)/safepool.a
	cp $(IOS_OUT)/arm64.h $(IOS_OUT)/safepool.h

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
	go build -v -buildmode=c-shared -o $(ANDROID_OUT)/arm64-v8a/libsafepool.so

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
	go build -v -buildmode=c-shared -o $(ANDROID_OUT)/x86_64/libsafepool.so

#android: android-armv7a android-arm64 android-x86 android-x86_64
android: android-arm64 android-x86_64
linux: linux-x86_64
