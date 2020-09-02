package lnd

/*
	struct Receive_return {
		char* r0;
		int r1;
		char* r2;
	};

	struct Receive_return receive_cgo(char* msg, int length, void* p, int id) {
	   struct Receive_return receive(char* msg, int length, void* p, int id);
	   return receive(msg, length, p, id);
	}

	char* send_cgo(void* p, int id) {
	   char* send(void* p, int id);
	   return send(p, id);
	}

	void* duplicate_cgo(void* p) {
		void* duplicate(void* p);
		return duplicate(p);
	}
*/
import "C"
