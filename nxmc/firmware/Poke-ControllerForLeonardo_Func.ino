#include "Poke-ControllerForLeonardo_Func.h"
#include "switch_controller_plus.h"
#include "UseKeyboardForNintendoSwitch_Func.h"
//#include "SoftwareReset.hpp"

#define YEAR_MAX 60

volatile int Command_index = 0;

typedef enum {
  SWITCH_Y       = 0x01,
  SWITCH_B       = 0x02,
  SWITCH_A       = 0x04,
  SWITCH_X       = 0x08,
  SWITCH_L       = 0x10,
  SWITCH_R       = 0x20,
  SWITCH_ZL      = 0x40,
  SWITCH_ZR      = 0x80,
  SWITCH_MINUS   = 0x100,
  SWITCH_PLUS    = 0x200,
  SWITCH_LCLICK  = 0x400,
  SWITCH_RCLICK  = 0x800,
  SWITCH_HOME    = 0x1000,
  SWITCH_CAPTURE = 0x2000,
} JoystickButtons_t;

typedef enum {
  INIT,
  SYNC,
  PROCESS,
  CLEANUP,
  DONE
} State_t;
State_t state = INIT;

char* cmd_name[MAX_BUFFER] = {
  "none\r\n",
  "mash_a\r\n",
  "aaabb\r\n",
  "auto_league\r\n",
  "inf_watt\r\n",
  "pickupberry\r\n",
  "changethedate\r\n",
  "changetheyear\r\n",
  "p_sync\r\n",
  "p_unsync\r\n",
  "debug\r\n",
  "debug2\r\n",
  
  "pc_call\r\n",
  "pc_call_string\r\n",
  "pc_call_keyboard\r\n",
};

static int duration_count;
static int duration_buf;
static int step_size_buf;

uint8_t pc_lx, pc_ly, pc_rx, pc_ry;
uint32_t KeyValue;
uint32_t YearChangeCnt;//0~4294967295までの整数
uint32_t MonthChangeCnt;//0~4294967295までの整数
uint32_t DayChangeCnt;//0~4294967295までの整数
int NowYear = 0;


static unsigned long s_ultime;
static bool blduration = false;
static bool blwaittime = false;
static int cnt_command = 0;

void Serial_Init()
{
  Serial.begin(9600);
  while (!Serial);
}
void Serial1_Init()
{
  Serial1.begin(SERIAL1_BAUD_RATE);
  while (!Serial1);
}
void Controller_Init()
{
  ResetDirections();//送信するデータをリセットする
  /* 認識させるため、複数回Switchにデータを送信する */
  for (int i = 0; i < 5; i++)
  {
    sendReportOnly(pc_report);//解析したデータをSwitchに送信
    delay(40);
  }
}
/* 送信するデータをリセットする */
void ResetDirections()
{
  pc_report.LX = 128;
  pc_report.LY = 128;
  pc_report.RX = 128;
  pc_report.RY = 128;
  pc_report.Hat = HAT_CENTER;
}

void ParseLine(char* line)
{
  char cmd[16];
  uint16_t p_btns;
  uint8_t hat;
  // get command
  int ret = sscanf(line, "%s", cmd);
  if (ret == EOF) {
    proc_state = DEBUG;
  } else if (strncmp(cmd, "end", 16) == 0) {
    proc_state = NONE;
    ResetDirections();
  } else if (strncmp(cmd, "resetMcu", 16) == 0) {
    //softwareReset::standard();
    proc_state = NONE;
  } else if ((uint8_t)cmd[0] == 0xaa) {
    memset(&pc_report, 0, sizeof(uint16_t));
    pc_lx = STICK_CENTER;
    pc_ly = STICK_CENTER;
    pc_rx = STICK_CENTER;
    pc_ry = STICK_CENTER;
    ResetDirections();
    uint8_t* data = (uint8_t*)line;
    p_btns = data[5] | ((uint16_t)data[6] << 8);
    hat = data[7];
    if (data[8] & 1) pc_lx = STICK_MIN;
    if (data[8] & 2) pc_lx = STICK_MAX;
    if (data[8] & 4) pc_ly = STICK_MIN;
    if (data[8] & 8) pc_ly = STICK_MAX;
    if (data[9] & 1) pc_rx = STICK_MIN;
    if (data[9] & 2) pc_rx = STICK_MAX;
    if (data[9] & 4) pc_ry = STICK_MIN;
    if (data[9] & 8) pc_ry = STICK_MAX;

    pc_report.Button = p_btns;
    pc_report.Hat = hat;
    pc_report.LX = pc_lx;
    pc_report.LY = pc_ly;
    pc_report.RX = pc_rx;
    pc_report.RY = pc_ry;

    proc_state = PC_CALL;
  } else if ((uint8_t)cmd[0] == 0xab) {
    memset(&pc_report, 0, sizeof(uint16_t));
    uint8_t* data = (uint8_t*)line;
    p_btns = data[1] | ((uint16_t)data[2] << 8);
    hat = data[3];
    pc_lx = data[4];
    pc_ly = data[5];
    pc_rx = data[6];
    pc_ry = data[7];

    pc_report.Button = p_btns;
    pc_report.Hat = hat;
    pc_report.LX = pc_lx;
    pc_report.LY = pc_ly;
    pc_report.RX = pc_rx;
    pc_report.RY = pc_ry;

    // keyboard
    if (data[8] == 1) {
      // normal press
      BootKeyboard.press((char)data[9]);
    } else if (data[8] == 2) {
      // normal release
      BootKeyboard.release((char)data[9]);
    } else if (data[8] == 3) {
      // special press
      BootKeyboard_PressSpecialKey(data[9]);
    } else if (data[8] == 4) {
      // special release
      BootKeyboard_ReleaseSpecialKey(data[9]);
    } else if (data[8] == 5) {
      // all release
      BootKeyboard.releaseAll();
    }
    proc_state = PC_CALL;
  } else if (cmd[0] >= '0' && cmd[0] <= '9') {
    memset(&pc_report, 0, sizeof(uint16_t));

    // format [button LeftStickX LeftStickY RightStickX RightStickY HAT]
    // button: Y | B | A | X | L | R | ZL | ZR | MINUS | PLUS | LCLICK | RCLICK | HOME | CAP
    // LeftStick : 0 to 255
    // RightStick: 0 to 255
    sscanf(line, "%hx %hhx %hhx %hhx %hhx %hhx", &p_btns, &hat,
           &pc_lx, &pc_ly, &pc_rx, &pc_ry);

    // HAT : 0(TOP) to 7(TOP_LEFT) in clockwise | 8(CENTER)
    pc_report.Hat = hat;

    // we use bit array for buttons(2 Bytes), which last 2 bits are flags of directions
    bool use_right = p_btns & 0x1;
    bool use_left = p_btns & 0x2;

    // Left stick
    if (use_left) {
      pc_report.LX = pc_lx;
      pc_report.LY = pc_ly;
    }

    // Right stick
    if (use_right & use_left) {
      pc_report.RX = pc_rx;
      pc_report.RY = pc_ry;
    } else if (use_right) {
      pc_report.RX = pc_lx;
      pc_report.RY = pc_ly;
    }

    p_btns >>= 2;
    pc_report.Button |= p_btns;

    proc_state = PC_CALL;
  } else if (strncmp(line, "\"", 1) == 0) {
    for (int i = 0; i < MAX_BUFFER; i++)
    {
      chrread[i] = (char)pc_report_str[i];
    }
    Type_stringBySerialCommunication();//シリアル通信を用いてキーボードを使用する
    memset(chrread, 0, sizeof(chrread));
    proc_state = PC_CALL_STRING;
  } else if (strncmp(cmd, "Key", 3) == 0) {
    sscanf(line, "Key %lu", &KeyValue);
    BootKeyboard_WriteSpecialKey(KeyValue);
    proc_state = PC_CALL_KEYBOARD;
    //ProgState = STATE1;
  } else if (strncmp(cmd, "Press", 5) == 0) {
    sscanf(line, "Press %lu", &KeyValue);
    BootKeyboard_PressSpecialKey(KeyValue);
    proc_state = PC_CALL_KEYBOARD_PRESS;
    //ProgState = STATE1;
  } else if (strncmp(cmd, "Release", 7) == 0) {
    sscanf(line, "Release %lu", &KeyValue);
    BootKeyboard_ReleaseSpecialKey(KeyValue);
    proc_state = PC_CALL_KEYBOARD_RELEASE;
    //ProgState = STATE1;
  //} else if (strncmp(cmd, cmd_name[MASH_A], 6) == 0) {
  //  proc_state = MASH_A;
  //   ProgState = STATE1;
  //} else if (strncmp(cmd, cmd_name[AAABB], 5) == 0) {
  //  proc_state = AAABB;
  //  ProgState = STATE1;
  //} else if (strncmp(cmd, cmd_name[AUTO_LEAGUE], 6) == 0) {
  //  proc_state = AUTO_LEAGUE;
  //  ProgState = STATE1;
  //} else if (strncmp(cmd, cmd_name[INF_WATT], 6) == 0) {
  //  proc_state = INF_WATT;
  //  ProgState = STATE1;
  //} else if (strncmp(cmd, cmd_name[PICKUPBERRY], 6) == 0) {
  //  proc_state = PICKUPBERRY;
  //  ProgState = STATE1;
  //} else if (strncmp(cmd, "Date", 4) == 0) {
  //  sscanf(line, "Date %lu/%lu/%lu", &YearChangeCnt, &MonthChangeCnt, &DayChangeCnt);
  //  proc_state = CHANGETHEDATE;
  //  ProgState = STATE1;
  //} else if (strncmp(cmd, "Year", 4) == 0) {
  //  proc_state = CHANGETHEYEAR;
  //  sscanf(line, "Year %lu", &YearChangeCnt);
  //  NowYear = 0;
  //} else if (strncmp(cmd, cmd_name[P_SYNC], 6) == 0) {
  //  proc_state = P_SYNC;
  //} else if (strncmp(cmd, cmd_name[P_UNSYNC], 6) == 0) {
  //  proc_state = P_UNSYNC;
  } else {
    proc_state = DEBUG2;
  }
  Serial1.println(proc_state, DEC);
  cnt_command = 0;
  step_size_buf = INT8_MAX;
  duration_buf = 0;
}

void GetNextReportFromCommands(SetCommand* commands, const int step_size)
{
  if ((blduration == false) && (blwaittime == false))
  {
    memcpy(&last_pc_report, &pc_report,  sizeof(USB_JoystickReport_Input_t));
    ApplyButtonCommand(commands, pc_report);
    sendReportOnly(pc_report);//解析したデータをSwitchに送信
    s_ultime = millis();
    blduration = true;
    blwaittime = true;
    return;
  }
  else if ((blduration == true) && (blwaittime == true))
  {
    if (millis() - s_ultime > commands[cnt_command].duration)
    {
      sendReportOnly(last_pc_report);//解析したデータをSwitchに送信
      s_ultime = millis();
      blduration = false;
    }
    return;
  }
  else
  {
    if (millis() - s_ultime > commands[cnt_command].waittime)
    {

      memcpy(&pc_report, &last_pc_report, sizeof(USB_JoystickReport_Input_t));
      cnt_command++;
      if (cnt_command >= step_size)
      {
        cnt_command = 0;
      }
      blduration = false;
      blwaittime = false;
    }
  }
}

void GetNextReportFromCommandsforChangeTheDate(SetCommand* commands, const int step_size)
{
  if ((blduration == false) && (blwaittime == false))
  {
    memcpy(&last_pc_report, &pc_report,  sizeof(USB_JoystickReport_Input_t));
    ApplyButtonCommand(commands, pc_report);
    sendReportOnly(pc_report);//解析したデータをSwitchに送信
    s_ultime = millis();
    blduration = true;
    blwaittime = true;
    return;
  }
  else if ((blduration == true) && (blwaittime == true))
  {
    if (millis() - s_ultime > commands[cnt_command].duration)
    {
      sendReportOnly(last_pc_report);//解析したデータをSwitchに送信
      s_ultime = millis();
      blduration = false;
    }
    return;
  }
  else
  {
    if (millis() - s_ultime > commands[cnt_command].waittime)
    {
      memcpy(&pc_report, &last_pc_report, sizeof(USB_JoystickReport_Input_t));
      cnt_command++;
      if(cnt_command >= step_size)
      {
        cnt_command = step_size - 1;
      }

      if(YearChangeCnt > 0)
      {
        if((cnt_command == INDEX_ARRAY_YEAR + 1) && (YearChangeCnt > 1))
        {
          cnt_command = INDEX_ARRAY_YEAR;
          YearChangeCnt--;
        }
      }
      else
      {
        if(cnt_command == INDEX_ARRAY_YEAR)
        {
          cnt_command++;
        }
      }

      if(MonthChangeCnt > 0)
      {
        if((cnt_command == INDEX_ARRAY_MONTH + 1) && (MonthChangeCnt > 1))
        {
          cnt_command = INDEX_ARRAY_MONTH;
          MonthChangeCnt--;
        }
      }
      else
      {
        if(cnt_command == INDEX_ARRAY_MONTH)
        {
          cnt_command++;
        }
      }

      if(DayChangeCnt > 0)
      {
        if((cnt_command == INDEX_ARRAY_DAY + 1) && (DayChangeCnt > 1))
        {
          cnt_command = INDEX_ARRAY_DAY;
          DayChangeCnt--;
        }
      }
      else
      {
        if(cnt_command == INDEX_ARRAY_DAY)
        {
          cnt_command++;
        }
      }
      blduration = false;
      blwaittime = false;
    }
  }
}

void GetNextReportFromCommandsforChangeTheYear(SetCommand* commands, const int step_size)
{
  if ((blduration == false) && (blwaittime == false))
  {
    memcpy(&last_pc_report, &pc_report,  sizeof(USB_JoystickReport_Input_t));
    ApplyButtonCommand(commands, pc_report);
    sendReportOnly(pc_report);//解析したデータをSwitchに送信
    s_ultime = millis();
    blduration = true;
    blwaittime = true;
    return;
  }
  else if ((blduration == true) && (blwaittime == true))
  {
    if (millis() - s_ultime > commands[cnt_command].duration)
    {
      sendReportOnly(last_pc_report);//解析したデータをSwitchに送信
      s_ultime = millis();
      blduration = false;
    }
    return;
  }
  else
  {
    if (millis() - s_ultime > commands[cnt_command].waittime)
    {

      memcpy(&pc_report, &last_pc_report, sizeof(USB_JoystickReport_Input_t));
      cnt_command++;

      if ((YearChangeCnt > 0) && (cnt_command == 14))
      {
        YearChangeCnt--;
        if (YearChangeCnt == 0)
        {
          cnt_command = 27;
        }
        else if (NowYear < YEAR_MAX - 1)
        {
          cnt_command = 1;
          NowYear++;
        }
        else
        {
          //何もしない
        }
      }
      else if ((YearChangeCnt > 0) && (cnt_command == 27))
      {
        cnt_command = 1;
        NowYear = 0;
      }
      else if ((YearChangeCnt == 0) && (cnt_command > step_size_buf - 1))
      {
        cnt_command = step_size_buf - 1;
      }
      else
      {
        //何もしない
      }

      blduration = false;
      blwaittime = false;
    }
  }
}

USB_JoystickReport_Input_t ApplyButtonCommand(SetCommand* commands, USB_JoystickReport_Input_t ReportData)
{
  byte button = commands[cnt_command].command;
  switch (button)
  {
    case COMMAND_UP:
      pc_report.LY = STICK_MIN;
      break;

    case COMMAND_LEFT:
      pc_report.LX = STICK_MIN;
      break;

    case COMMAND_DOWN:
      pc_report.LY = STICK_MAX;
      break;

    case COMMAND_RIGHT:
      pc_report.LX = STICK_MAX;
      break;

    case COMMAND_A:
      pc_report.Button |= SWITCH_A;
      break;

    case COMMAND_B:
      pc_report.Button |= SWITCH_B;
      break;

    case COMMAND_X:
      pc_report.Button |= SWITCH_X;
      break;

    case COMMAND_Y:
      pc_report.Button |= SWITCH_Y;
      break;

    case COMMAND_L:
      pc_report.Button |= SWITCH_L;
      break;

    case COMMAND_R:
      pc_report.Button |= SWITCH_R;
      break;

    case COMMAND_TRIGGERS:
      pc_report.Button |= SWITCH_L | SWITCH_R;
      break;

    case COMMAND_UPLEFT:
      pc_report.LX = STICK_MIN;
      pc_report.LY = STICK_MIN;
      break;

    case COMMAND_UPRIGHT:
      pc_report.LX = STICK_MAX;
      pc_report.LY = STICK_MIN;
      break;

    case COMMAND_DOWNRIGHT:
      pc_report.LX = STICK_MAX;
      pc_report.LY = STICK_MAX;
      break;

    case COMMAND_DOWNLEFT:
      pc_report.LX = STICK_MIN;
      pc_report.LY = STICK_MAX;
      break;

    case COMMAND_PLUS:
      pc_report.Button |= SWITCH_PLUS;
      break;

    case COMMAND_MINUS:
      pc_report.Button |= SWITCH_MINUS;
      break;

    case COMMAND_HOME:
      pc_report.Button |= SWITCH_HOME;
      break;

    case COMMAND_RS_UP:
      pc_report.RY = STICK_MIN;
      break;

    case COMMAND_RS_LEFT:
      pc_report.RX = STICK_MIN;
      break;

    case COMMAND_RS_DOWN:
      pc_report.RY = STICK_MAX;
      break;

    case COMMAND_RS_RIGHT:
      pc_report.RX = STICK_MAX;
      break;

    case COMMAND_RS_UPLEFT:
      pc_report.RX = STICK_MIN;
      pc_report.RY = STICK_MIN;
      break;

    case COMMAND_RS_UPRIGHT:
      pc_report.RX = STICK_MAX;
      pc_report.RY = STICK_MIN;
      break;

    case COMMAND_RS_DOWNRIGHT:
      pc_report.RX = STICK_MAX;
      pc_report.RY = STICK_MAX;
      break;

    case COMMAND_RS_DOWNLEFT:
      pc_report.RX = STICK_MIN;
      pc_report.RY = STICK_MAX;
      break;

    case COMMAND_HAT_TOP:
      pc_report.Hat |= HAT_TOP;
      break;

    case COMMAND_HAT_TOP_RIGHT:
      pc_report.Hat |= HAT_TOP_RIGHT;
      break;

    case COMMAND_HAT_BOTTOM_RIGHT:
      pc_report.Hat |= HAT_BOTTOM_RIGHT;
      break;

    case COMMAND_HAT_BOTTOM:
      pc_report.Hat |= HAT_BOTTOM;
      break;

    case COMMAND_HAT_BOTTOM_LEFT:
      pc_report.Hat |= HAT_BOTTOM_LEFT;
      break;

    case COMMAND_HAT_LEFT:
      pc_report.Hat |= HAT_LEFT;
      break;

    default:
      pc_report.LX = STICK_CENTER;
      pc_report.LY = STICK_CENTER;
      pc_report.RX = STICK_CENTER;
      pc_report.RY = STICK_CENTER;
      pc_report.Hat = HAT_CENTER;
      break;
  }
  return ReportData;
}
void SwitchFunction()
{
  switch (proc_state)
  {
    case PC_CALL:
      sendReportOnly(pc_report);//解析したデータをSwitchに送信
      ProgState = STATE0;
      break;
    case PC_CALL_STRING:
      Type_stringBySerialCommunication();//シリアル通信を用いてキーボードを使用する
      memset(chrread, 0, sizeof(chrread));
      ProgState = STATE0;
      break;
    case PC_CALL_KEYBOARD:
      BootKeyboard_WriteSpecialKey(KeyValue);
      ProgState = STATE0;
      break;
    case PC_CALL_KEYBOARD_PRESS:
      BootKeyboard_PressSpecialKey(KeyValue);
      ProgState = STATE0;
      break;
    case PC_CALL_KEYBOARD_RELEASE:
      BootKeyboard_ReleaseSpecialKey(KeyValue);
      ProgState = STATE0;
      break;
    case MASH_A:
      GetNextReportFromCommands(&mash_a_commands[0], mash_a_size);
      break;
    case AAABB:
      GetNextReportFromCommands(&aaabb_commands[0], aaabb_size);
      break;
    case AUTO_LEAGUE:
      pc_report.LX = 172;
      pc_report.LY = 7;
      GetNextReportFromCommands(&auto_league_commands[0], auto_league_size);
      break;
    case INF_WATT:
      GetNextReportFromCommands(&inf_watt_commands[0], inf_watt_size);
      break;
    case PICKUPBERRY:
      GetNextReportFromCommands(&pickupberry_commands[0], pickupberry_size);
      break;
    case CHANGETHEDATE:
      GetNextReportFromCommandsforChangeTheDate(&changethedate_commands[0], changethedate_size);
      break;
    case CHANGETHEYEAR:
      GetNextReportFromCommandsforChangeTheYear(&changetheyear_commands[0], changetheyear_size);
      break;
    default:
      /* 何もしない */
      break;
  }
}
