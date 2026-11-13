#include "BlueZLink.h"

#include "HAL/PlatformProcess.h"
#include "HAL/PlatformTime.h"
#include "HAL/Runnable.h"
#include "HAL/RunnableThread.h"
#include "Misc/ScopeLock.h"

#include <algorithm>
#include <atomic>
#include <map>
#include <string>
#include <type_traits>
#include <vector>

namespace
{
// ---- libdbus, loaded at run time ----------------------------------------------------------------
// Its types as far as this uses them. DBusError matches libdbus's layout; DBusMessageIter is only
// ever filled in by libdbus, so room enough (libdbus's is 72 bytes) and the alignment will do.
struct DBusConnection;
struct DBusMessage;
struct DBusError
{
	const char* Name;
	const char* Message;
	unsigned int Bits;
	void* Padding;
};
struct DBusMessageIter
{
	alignas(8) unsigned char Storage[128];
};
using dbus_bool_t = uint32;
constexpr int BusSystem = 1;
constexpr int DArray = 'a', DDict = 'e', DVariant = 'v', DString = 's', DPath = 'o', DBool = 'b', DByte = 'y';

struct FDBus
{
	void* Handle = nullptr;
	void (*ErrorInit)(DBusError*) = nullptr;
	void (*ErrorFree)(DBusError*) = nullptr;
	dbus_bool_t (*ThreadsInit)() = nullptr;
	DBusConnection* (*BusGetPrivate)(int, DBusError*) = nullptr;
	void (*BusAddMatch)(DBusConnection*, const char*, DBusError*) = nullptr;
	void (*Close)(DBusConnection*) = nullptr;
	void (*Unref)(DBusConnection*) = nullptr;
	void (*ExitOnDisconnect)(DBusConnection*, dbus_bool_t) = nullptr;
	DBusMessage* (*SendBlock)(DBusConnection*, DBusMessage*, int, DBusError*) = nullptr;
	dbus_bool_t (*ReadWrite)(DBusConnection*, int) = nullptr;
	DBusMessage* (*Pop)(DBusConnection*) = nullptr;
	DBusMessage* (*NewCall)(const char*, const char*, const char*, const char*) = nullptr;
	void (*MessageUnref)(DBusMessage*) = nullptr;
	dbus_bool_t (*IsSignal)(DBusMessage*, const char*, const char*) = nullptr;
	const char* (*GetPath)(DBusMessage*) = nullptr;
	dbus_bool_t (*IterInit)(DBusMessage*, DBusMessageIter*) = nullptr;
	void (*IterInitAppend)(DBusMessage*, DBusMessageIter*) = nullptr;
	int (*ArgType)(DBusMessageIter*) = nullptr;
	int (*ElementType)(DBusMessageIter*) = nullptr;
	void (*Recurse)(DBusMessageIter*, DBusMessageIter*) = nullptr;
	dbus_bool_t (*Next)(DBusMessageIter*) = nullptr;
	void (*GetBasic)(DBusMessageIter*, void*) = nullptr;
	void (*GetFixedArray)(DBusMessageIter*, void*, int*) = nullptr;
	dbus_bool_t (*Open)(DBusMessageIter*, int, const char*, DBusMessageIter*) = nullptr;
	dbus_bool_t (*CloseContainer)(DBusMessageIter*, DBusMessageIter*) = nullptr;
	dbus_bool_t (*AppendBasic)(DBusMessageIter*, int, const void*) = nullptr;
	dbus_bool_t (*AppendFixedArray)(DBusMessageIter*, int, const void*, int) = nullptr;

	bool Load()
	{
		Handle = FPlatformProcess::GetDllHandle(TEXT("libdbus-1.so.3"));
		if (!Handle)
		{
			return false;
		}
		bool bAll = true;
		auto Get = [&](auto& Field, const TCHAR* Symbol)
		{
			Field = reinterpret_cast<std::remove_reference_t<decltype(Field)>>(FPlatformProcess::GetDllExport(Handle, Symbol));
			bAll = bAll && Field;
		};
		Get(ErrorInit, TEXT("dbus_error_init"));
		Get(ErrorFree, TEXT("dbus_error_free"));
		Get(ThreadsInit, TEXT("dbus_threads_init_default"));
		Get(BusGetPrivate, TEXT("dbus_bus_get_private"));
		Get(BusAddMatch, TEXT("dbus_bus_add_match"));
		Get(Close, TEXT("dbus_connection_close"));
		Get(Unref, TEXT("dbus_connection_unref"));
		Get(ExitOnDisconnect, TEXT("dbus_connection_set_exit_on_disconnect"));
		Get(SendBlock, TEXT("dbus_connection_send_with_reply_and_block"));
		Get(ReadWrite, TEXT("dbus_connection_read_write"));
		Get(Pop, TEXT("dbus_connection_pop_message"));
		Get(NewCall, TEXT("dbus_message_new_method_call"));
		Get(MessageUnref, TEXT("dbus_message_unref"));
		Get(IsSignal, TEXT("dbus_message_is_signal"));
		Get(GetPath, TEXT("dbus_message_get_path"));
		Get(IterInit, TEXT("dbus_message_iter_init"));
		Get(IterInitAppend, TEXT("dbus_message_iter_init_append"));
		Get(ArgType, TEXT("dbus_message_iter_get_arg_type"));
		Get(ElementType, TEXT("dbus_message_iter_get_element_type"));
		Get(Recurse, TEXT("dbus_message_iter_recurse"));
		Get(Next, TEXT("dbus_message_iter_next"));
		Get(GetBasic, TEXT("dbus_message_iter_get_basic"));
		Get(GetFixedArray, TEXT("dbus_message_iter_get_fixed_array"));
		Get(Open, TEXT("dbus_message_iter_open_container"));
		Get(CloseContainer, TEXT("dbus_message_iter_close_container"));
		Get(AppendBasic, TEXT("dbus_message_iter_append_basic"));
		Get(AppendFixedArray, TEXT("dbus_message_iter_append_fixed_array"));
		return bAll;
	}
};

// A 16-bit SIG UUID from BlueZ's "0000xxxx-0000-1000-8000-00805f9b34fb", or 0 for any other.
uint16 ShortUuid(const std::string& Uuid)
{
	static const std::string Base = "-0000-1000-8000-00805f9b34fb";
	if (Uuid.size() != 36 || Uuid.compare(0, 4, "0000") != 0 || Uuid.compare(8, std::string::npos, Base) != 0)
	{
		return 0;
	}
	return static_cast<uint16>(std::stoul(Uuid.substr(4, 4), nullptr, 16));
}

// One object's properties, as far as this reads them.
struct FProps
{
	std::string Name, Alias, Uuid, Service, Device;
	std::vector<uint16> Uuids;
	bool bHasConnected = false, bConnected = false, bResolved = false;
	bool bHasValue = false;
	Veloverse::Ble::Bytes Value;
};

struct FDevice
{
	std::string Path, Name;
	std::vector<uint16> Services;
	bool bConnected = false, bResolved = false;
	bool Offers(uint16 Service) const { return std::find(Services.begin(), Services.end(), Service) != Services.end(); }
};

struct FCharacteristic
{
	std::string Path, Device;
	uint16 Uuid = 0;
};

// The characteristics read: the trainer's, a power meter's, a cadence sensor's and a strap's.
bool Wanted(uint16 Uuid)
{
	using namespace Veloverse::Ble;
	return Uuid == IndoorBikeData || Uuid == ControlPoint || Uuid == HeartRateMeasurement ||
	       Uuid == CyclingPowerMeasurement || Uuid == SpeedCadenceMeasurement;
}
} // namespace

struct FBlueZLink::FImpl : public FRunnable
{
	FDBus Bus;
	DBusConnection* Conn = nullptr;
	FRunnableThread* Thread = nullptr;
	std::atomic<bool> bStop{false};
	std::atomic<bool> bTrainer{false};

	// Shared with the game thread, under Lock.
	mutable FCriticalSection Lock;
	FString Status = TEXT("starting");
	TArray<FString> Found;
	FString Choice; // the trainer picked, by name; empty for the first found
	TArray<FSensor> Sensors;
	FString StrapChoice; // the strap picked, likewise
	TArray<TPair<uint16, Veloverse::Ble::Bytes>> Inbox;
	TArray<Veloverse::Ble::Bytes> Outbox;

	// The worker's own.
	std::string Adapter;
	bool bDiscovering = false;
	std::vector<FDevice> Known;
	std::vector<FCharacteristic> Characteristics;
	std::string TrainerPath, StrapPath;
	std::map<std::string, uint16> Subscribed; // characteristic path to its UUID
	std::string ControlPath;
	std::map<std::string, double> LastConnect; // device path to when a connect was last tried
	FString LoggedStatus;

	FImpl() { Thread = FRunnableThread::Create(this, TEXT("BlueZ"), 0, TPri_BelowNormal); }

	virtual ~FImpl() override
	{
		bStop = true;
		if (Thread)
		{
			Thread->WaitForCompletion();
			delete Thread;
		}
	}

	void SetStatus(const FString& Now)
	{
		{
			FScopeLock Guard(&Lock);
			Status = Now;
		}
		if (Now != LoggedStatus)
		{
			LoggedStatus = Now;
			UE_LOG(LogTemp, Display, TEXT("RIDE ble: %s"), *Now);
		}
	}

	virtual uint32 Run() override
	{
		if (!Bus.Load())
		{
			SetStatus(TEXT("no D-Bus (libdbus-1 isn't installed)"));
			return 0;
		}
		Bus.ThreadsInit();
		DBusError Error;
		Bus.ErrorInit(&Error);
		Conn = Bus.BusGetPrivate(BusSystem, &Error);
		if (!Conn)
		{
			SetStatus(FString::Printf(TEXT("no D-Bus system bus (%hs)"), Error.Message ? Error.Message : "no reason given"));
			Bus.ErrorFree(&Error);
			return 0;
		}
		Bus.ExitOnDisconnect(Conn, 0);
		Bus.BusAddMatch(Conn,
		                "type='signal',sender='org.bluez',interface='org.freedesktop.DBus.Properties',"
		                "member='PropertiesChanged'",
		                &Error);
		Bus.ErrorFree(&Error);
		double Surveyed = -1e9;
		while (!bStop)
		{
			const double Now = FPlatformTime::Seconds();
			if (Now - Surveyed > 2.0)
			{
				Surveyed = Now;
				Survey(Now);
			}
			Pump();
			Send();
		}
		// Let the trainer go, so another app can have it.
		for (const std::string& Path : {TrainerPath, StrapPath})
		{
			if (!Path.empty())
			{
				Release(Call(Path.c_str(), "org.bluez.Device1", "Disconnect", nullptr, 3000));
			}
		}
		if (bDiscovering)
		{
			Release(Call(Adapter.c_str(), "org.bluez.Adapter1", "StopDiscovery", nullptr, 2000));
		}
		Bus.Close(Conn);
		Bus.Unref(Conn);
		Conn = nullptr;
		return 0;
	}

	virtual void Stop() override { bStop = true; }

	// ---- Calls ----------------------------------------------------------------------------------

	DBusMessage* Call(const char* Path, const char* Interface, const char* Method,
	                  TFunctionRef<void(DBusMessageIter&)>* Args = nullptr, int TimeoutMs = 5000,
	                  std::string* Failure = nullptr)
	{
		DBusMessage* M = Bus.NewCall("org.bluez", Path, Interface, Method);
		if (!M)
		{
			return nullptr;
		}
		if (Args)
		{
			DBusMessageIter It;
			Bus.IterInitAppend(M, &It);
			(*Args)(It);
		}
		DBusError Error;
		Bus.ErrorInit(&Error);
		DBusMessage* Reply = Bus.SendBlock(Conn, M, TimeoutMs, &Error);
		Bus.MessageUnref(M);
		if (!Reply && Failure)
		{
			*Failure = Error.Message ? Error.Message : (Error.Name ? Error.Name : "failed");
		}
		Bus.ErrorFree(&Error);
		return Reply;
	}

	void Release(DBusMessage* M)
	{
		if (M)
		{
			Bus.MessageUnref(M);
		}
	}

	// ---- Reading replies ------------------------------------------------------------------------

	std::string String(DBusMessageIter& It)
	{
		const char* S = nullptr;
		Bus.GetBasic(&It, &S);
		return S ? S : "";
	}

	// A variant's value into the property it is.
	void Variant(DBusMessageIter& It, const std::string& Key, FProps& Out)
	{
		DBusMessageIter V;
		Bus.Recurse(&It, &V);
		const int Type = Bus.ArgType(&V);
		if (Type == DString || Type == DPath)
		{
			const std::string S = String(V);
			if (Key == "Name") Out.Name = S;
			else if (Key == "Alias") Out.Alias = S;
			else if (Key == "UUID") Out.Uuid = S;
			else if (Key == "Service") Out.Service = S;
			else if (Key == "Device") Out.Device = S;
		}
		else if (Type == DBool)
		{
			dbus_bool_t B = 0;
			Bus.GetBasic(&V, &B);
			if (Key == "Connected")
			{
				Out.bHasConnected = true;
				Out.bConnected = B != 0;
			}
			else if (Key == "ServicesResolved")
			{
				Out.bResolved = B != 0;
			}
		}
		else if (Type == DArray && Bus.ElementType(&V) == DByte && Key == "Value")
		{
			DBusMessageIter A;
			Bus.Recurse(&V, &A);
			const uint8* Data = nullptr;
			int N = 0;
			Bus.GetFixedArray(&A, &Data, &N);
			Out.bHasValue = true;
			Out.Value.assign(Data, Data + FMath::Max(N, 0));
		}
		else if (Type == DArray && Bus.ElementType(&V) == DString && Key == "UUIDs")
		{
			DBusMessageIter A;
			Bus.Recurse(&V, &A);
			while (Bus.ArgType(&A) == DString)
			{
				if (const uint16 U = ShortUuid(String(A)))
				{
					Out.Uuids.push_back(U);
				}
				Bus.Next(&A);
			}
		}
	}

	// a{sv}
	FProps Properties(DBusMessageIter& Dict)
	{
		FProps Out;
		DBusMessageIter Entries;
		Bus.Recurse(&Dict, &Entries);
		while (Bus.ArgType(&Entries) == DDict)
		{
			DBusMessageIter Entry;
			Bus.Recurse(&Entries, &Entry);
			const std::string Key = String(Entry);
			Bus.Next(&Entry);
			if (Bus.ArgType(&Entry) == DVariant)
			{
				Variant(Entry, Key, Out);
			}
			Bus.Next(&Entries);
		}
		return Out;
	}

	// Everything BlueZ knows: the adapter, the devices and their characteristics.
	bool ReadObjects()
	{
		DBusMessage* M = Bus.NewCall("org.bluez", "/", "org.freedesktop.DBus.ObjectManager", "GetManagedObjects");
		if (!M)
		{
			return false;
		}
		DBusError Error;
		Bus.ErrorInit(&Error);
		DBusMessage* Reply = Bus.SendBlock(Conn, M, 5000, &Error);
		Bus.MessageUnref(M);
		if (!Reply)
		{
			SetStatus(FString::Printf(TEXT("BlueZ isn't running (%hs)"), Error.Message ? Error.Message : "no reply"));
			Bus.ErrorFree(&Error);
			return false;
		}
		Bus.ErrorFree(&Error);
		Adapter.clear();
		Known.clear();
		Characteristics.clear();
		DBusMessageIter Root, Objects;
		if (Bus.IterInit(Reply, &Root) && Bus.ArgType(&Root) == DArray)
		{
			Bus.Recurse(&Root, &Objects);
			while (Bus.ArgType(&Objects) == DDict)
			{
				DBusMessageIter Object, Interfaces;
				Bus.Recurse(&Objects, &Object);
				const std::string Path = String(Object);
				Bus.Next(&Object);
				Bus.Recurse(&Object, &Interfaces);
				while (Bus.ArgType(&Interfaces) == DDict)
				{
					DBusMessageIter Interface;
					Bus.Recurse(&Interfaces, &Interface);
					const std::string Name = String(Interface);
					Bus.Next(&Interface);
					if (Name == "org.bluez.Adapter1" && Adapter.empty())
					{
						Adapter = Path;
					}
					else if (Name == "org.bluez.Device1")
					{
						const FProps P = Properties(Interface);
						FDevice D;
						D.Path = Path;
						D.Name = !P.Name.empty() ? P.Name : !P.Alias.empty() ? P.Alias : Path.substr(Path.rfind('/') + 1);
						D.Services = P.Uuids;
						D.bConnected = P.bConnected;
						D.bResolved = P.bResolved;
						Known.push_back(MoveTemp(D));
					}
					else if (Name == "org.bluez.GattCharacteristic1")
					{
						const FProps P = Properties(Interface);
						const uint16 U = ShortUuid(P.Uuid);
						if (Wanted(U))
						{
							// /org/bluez/hci0/dev_XX/service000a/char000b: the device is two levels up.
							const std::string Service = Path.substr(0, Path.rfind('/'));
							Characteristics.push_back({Path, Service.substr(0, Service.rfind('/')), U});
						}
					}
					Bus.Next(&Interfaces);
				}
				Bus.Next(&Objects);
			}
		}
		Bus.MessageUnref(Reply);
		return true;
	}

	// ---- The connection ------------------------------------------------------------------------

	const FDevice* Device(const std::string& Path) const
	{
		for (const FDevice& D : Known)
		{
			if (D.Path == Path)
			{
				return &D;
			}
		}
		return nullptr;
	}

	void Survey(double Now)
	{
		using namespace Veloverse::Ble;
		if (!ReadObjects())
		{
			return;
		}
		if (Adapter.empty())
		{
			SetStatus(TEXT("no Bluetooth adapter"));
			return;
		}
		if (!bDiscovering)
		{
			std::string Failure;
			DBusMessage* R = Call(Adapter.c_str(), "org.bluez.Adapter1", "StartDiscovery", nullptr, 5000, &Failure);
			bDiscovering = R != nullptr || Failure.find("progress") != std::string::npos;
			Release(R);
			if (!bDiscovering)
			{
				SetStatus(FString::Printf(TEXT("can't scan (%hs)"), Failure.c_str()));
				return;
			}
		}
		// The trainers on offer: fitness machines first, then power meters.
		TArray<FString> Names;
		std::vector<const FDevice*> Trainers;
		for (const uint16 Kind : {FitnessMachineService, CyclingPowerService})
		{
			for (const FDevice& D : Known)
			{
				if (D.Offers(Kind) && std::find(Trainers.begin(), Trainers.end(), &D) == Trainers.end())
				{
					Trainers.push_back(&D);
					Names.Add(UTF8_TO_TCHAR(D.Name.c_str()));
				}
			}
		}
		FString Want, WantStrap;
		{
			FScopeLock Guard(&Lock);
			Found = Names;
			Want = Choice;
			WantStrap = StrapChoice;
		}
		const FDevice* Chosen = nullptr;
		for (const FDevice* D : Trainers)
		{
			if (Want.IsEmpty() ? true : Want == UTF8_TO_TCHAR(D->Name.c_str()))
			{
				Chosen = D;
				break;
			}
		}
		if (!Chosen)
		{
			LetGo(TrainerPath);
		}
		else if (Chosen->Path != TrainerPath)
		{
			LetGo(TrainerPath); // another was picked
			TrainerPath = Chosen->Path;
		}
		// And a heart-rate strap, when there's one besides the trainer: the one picked, or the first.
		if (!StrapPath.empty() && !Device(StrapPath))
		{
			StrapPath.clear(); // gone
		}
		for (const FDevice& D : Known)
		{
			const bool bWanted = WantStrap.IsEmpty() ? StrapPath.empty() || StrapPath == D.Path
			                                         : WantStrap == UTF8_TO_TCHAR(D.Name.c_str());
			if (D.Offers(HeartRateService) && D.Path != TrainerPath && bWanted)
			{
				if (StrapPath != D.Path)
				{
					LetGo(StrapPath);
					StrapPath = D.Path;
				}
				Join(D, Now);
				break;
			}
		}
		PublishSensors(Trainers);
		if (!Chosen)
		{
			SetStatus(Want.IsEmpty() ? FString(TEXT("looking for a trainer")) : FString::Printf(TEXT("looking for %s"), *Want));
			return;
		}
		Join(*Chosen, Now);
		const FString Name = UTF8_TO_TCHAR(Chosen->Name.c_str());
		if (!Chosen->bConnected)
		{
			SetStatus(FString::Printf(TEXT("connecting to %s"), *Name));
		}
		else if (!bTrainer)
		{
			SetStatus(FString::Printf(TEXT("%s, power only (no control)"), *Name));
		}
		else
		{
			SetStatus(Name);
		}
	}

	// Tells the game thread what's on offer: the trainers in the order Survey ranks them, then the
	// straps.
	void PublishSensors(const std::vector<const FDevice*>& Trainers)
	{
		using namespace Veloverse::Ble;
		TArray<FSensor> List;
		for (const FDevice* D : Trainers)
		{
			const FSensor::EKind Kind = D->Offers(FitnessMachineService) ? FSensor::EKind::Trainer : FSensor::EKind::Power;
			List.Add({UTF8_TO_TCHAR(D->Name.c_str()), Kind, D->Path == TrainerPath, D->bConnected});
		}
		for (const FDevice& D : Known)
		{
			if (D.Offers(HeartRateService) && std::find(Trainers.begin(), Trainers.end(), &D) == Trainers.end())
			{
				List.Add({UTF8_TO_TCHAR(D.Name.c_str()), FSensor::EKind::Heart, D.Path == StrapPath, D.bConnected});
			}
		}
		FScopeLock Guard(&Lock);
		Sensors = MoveTemp(List);
	}

	// Connects a device and, once its services are known, subscribes to what's read from it.
	void Join(const FDevice& D, double Now)
	{
		if (!D.bConnected)
		{
			Forget(D.Path);
			double& Tried = LastConnect[D.Path];
			if (Now - Tried >= 10.0)
			{
				Tried = Now;
				std::string Failure;
				Release(Call(D.Path.c_str(), "org.bluez.Device1", "Connect", nullptr, 10000, &Failure));
				UE_CLOG(!Failure.empty(), LogTemp, Display, TEXT("RIDE ble: connecting %hs: %hs"), D.Name.c_str(), Failure.c_str());
			}
			return;
		}
		if (!D.bResolved)
		{
			return;
		}
		for (const FCharacteristic& C : Characteristics)
		{
			if (C.Device != D.Path || Subscribed.count(C.Path))
			{
				continue;
			}
			std::string Failure;
			DBusMessage* R = Call(C.Path.c_str(), "org.bluez.GattCharacteristic1", "StartNotify", nullptr, 5000, &Failure);
			if (R || Failure.find("lready") != std::string::npos) // "Already notifying" will do
			{
				Subscribed[C.Path] = C.Uuid;
				UE_LOG(LogTemp, Display, TEXT("RIDE ble: %hs notifies 0x%04X"), D.Name.c_str(), C.Uuid);
				if (C.Uuid == Veloverse::Ble::ControlPoint && D.Path == TrainerPath)
				{
					ControlPath = C.Path;
					bTrainer = true;
				}
			}
			Release(R);
		}
	}

	// A device gone or let go: nothing from it is read, and a trainer can't be steered.
	void Forget(const std::string& DevicePath)
	{
		for (auto It = Subscribed.begin(); It != Subscribed.end();)
		{
			It = It->first.rfind(DevicePath + "/", 0) == 0 ? Subscribed.erase(It) : std::next(It);
		}
		if (DevicePath == TrainerPath)
		{
			ControlPath.clear();
			bTrainer = false;
		}
	}

	void LetGo(std::string& DevicePath)
	{
		if (DevicePath.empty())
		{
			return;
		}
		Forget(DevicePath);
		Release(Call(DevicePath.c_str(), "org.bluez.Device1", "Disconnect", nullptr, 3000));
		DevicePath.clear();
	}

	// Notifications and control point replies, as BlueZ reports a characteristic's new value.
	void Pump()
	{
		Bus.ReadWrite(Conn, 100);
		while (DBusMessage* M = Bus.Pop(Conn))
		{
			if (Bus.IsSignal(M, "org.freedesktop.DBus.Properties", "PropertiesChanged"))
			{
				const char* Path = Bus.GetPath(M);
				const auto Sub = Path ? Subscribed.find(Path) : Subscribed.end();
				DBusMessageIter It;
				if (Sub != Subscribed.end() && Bus.IterInit(M, &It) && Bus.ArgType(&It) == DString)
				{
					Bus.Next(&It); // past the interface name, to the changed properties
					if (Bus.ArgType(&It) == DArray)
					{
						FProps P = Properties(It);
						if (P.bHasValue)
						{
							FScopeLock Guard(&Lock);
							Inbox.Add({Sub->second, MoveTemp(P.Value)});
						}
					}
				}
			}
			Bus.MessageUnref(M);
		}
	}

	// Control point writes, with a response, so a lost one is known.
	void Send()
	{
		TArray<Veloverse::Ble::Bytes> Writes;
		{
			FScopeLock Guard(&Lock);
			Swap(Writes, Outbox);
		}
		for (const Veloverse::Ble::Bytes& Value : Writes)
		{
			if (ControlPath.empty() || Value.empty())
			{
				continue;
			}
			auto Fill = [this, &Value](DBusMessageIter& It)
			{
				DBusMessageIter Array, Options, Entry, Var;
				Bus.Open(&It, DArray, "y", &Array);
				const uint8* Data = Value.data();
				Bus.AppendFixedArray(&Array, DByte, &Data, static_cast<int>(Value.size()));
				Bus.CloseContainer(&It, &Array);
				Bus.Open(&It, DArray, "{sv}", &Options);
				Bus.Open(&Options, DDict, nullptr, &Entry);
				const char* Key = "type";
				Bus.AppendBasic(&Entry, DString, &Key);
				Bus.Open(&Entry, DVariant, "s", &Var);
				const char* Kind = "request";
				Bus.AppendBasic(&Var, DString, &Kind);
				Bus.CloseContainer(&Entry, &Var);
				Bus.CloseContainer(&Options, &Entry);
				Bus.CloseContainer(&It, &Options);
			};
			TFunctionRef<void(DBusMessageIter&)> Args(Fill);
			std::string Failure;
			Release(Call(ControlPath.c_str(), "org.bluez.GattCharacteristic1", "WriteValue", &Args, 3000, &Failure));
			UE_CLOG(!Failure.empty(), LogTemp, Display, TEXT("RIDE ble: control point write: %hs"), Failure.c_str());
		}
	}
};

FBlueZLink::FBlueZLink() : Impl(MakeUnique<FImpl>()) {}

FBlueZLink::~FBlueZLink() = default;

FString FBlueZLink::Name() const
{
	FScopeLock Guard(&Impl->Lock);
	return TEXT("Bluetooth: ") + Impl->Status;
}

void FBlueZLink::Poll(Veloverse::Ble::FSession& Session, double Now)
{
	TArray<TPair<uint16, Veloverse::Ble::Bytes>> Got;
	{
		FScopeLock Guard(&Impl->Lock);
		Swap(Got, Impl->Inbox);
	}
	for (const TPair<uint16, Veloverse::Ble::Bytes>& Each : Got)
	{
		if (Each.Key == Veloverse::Ble::ControlPoint)
		{
			Session.Reply(Each.Value);
		}
		else
		{
			Session.Notify(Each.Key, Each.Value, Now);
		}
	}
}

void FBlueZLink::Write(const Veloverse::Ble::Bytes& ControlPoint)
{
	FScopeLock Guard(&Impl->Lock);
	Impl->Outbox.Add(ControlPoint);
}

bool FBlueZLink::HasTrainer() const { return Impl->bTrainer; }

TArray<FString> FBlueZLink::Devices() const
{
	FScopeLock Guard(&Impl->Lock);
	return Impl->Found;
}

FString FBlueZLink::Picked() const
{
	FScopeLock Guard(&Impl->Lock);
	return Impl->Choice;
}

void FBlueZLink::Pick(const FString& Device)
{
	FScopeLock Guard(&Impl->Lock);
	Impl->Choice = Device;
}

TArray<FSensor> FBlueZLink::Sensors() const
{
	FScopeLock Guard(&Impl->Lock);
	return Impl->Sensors;
}

void FBlueZLink::PickStrap(const FString& Device)
{
	FScopeLock Guard(&Impl->Lock);
	Impl->StrapChoice = Device;
}
