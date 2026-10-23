#!/usr/bin/env python3
"""Research-only Bitunix USDT perpetual multi-timeframe signal backtest.

Uses public Bitunix futures kline and historical funding-rate endpoints. It sends
NO private/authenticated requests and cannot place orders. Python deps: pandas,
requests, numpy. Results are an unleveraged research approximation, not execution
or liquidation simulation.
"""
from __future__ import annotations
import argparse, csv, json, math, os, time
from datetime import datetime, timedelta, timezone
from pathlib import Path
from typing import Any
import requests
import pandas as pd
import numpy as np

BASE = "https://fapi.bitunix.com"
KLINE = "/api/v1/futures/market/kline"
FUNDING = "/api/v1/futures/market/get_funding_rate_history"
TF_MS = {"1m":60_000,"3m":180_000,"5m":300_000,"15m":900_000}
STRATEGIES = ["EMA","MA","RSI","MACD","VOLUME","MOMENTUM","ATR_BREAKOUT","SUPERTREND","BOLLINGER"]
DATA_DIR = Path(__file__).parent / "data" / "bitunix"
OUT_DIR = Path(__file__).parent / "reports" / "bitunix"

class APIError(RuntimeError): pass

def api_get(path: str, params: dict[str,Any], session: requests.Session) -> dict:
    for attempt in range(5):
        r=session.get(BASE+path,params=params,timeout=25)
        if r.status_code == 429:
            time.sleep(1.0 + attempt); continue
        r.raise_for_status()
        body=r.json()
        if body.get("code") != 0:
            raise APIError(f"Bitunix API code={body.get('code')}: {body.get('msg','no message')}")
        return body
    raise APIError("Bitunix rate limit persisted after retries")

def fetch_klines(symbol: str, interval: str, start_ms: int, end_ms: int, refresh=False) -> pd.DataFrame:
    DATA_DIR.mkdir(parents=True,exist_ok=True)
    path=DATA_DIR/f"{symbol}_{interval}_LAST_PRICE.csv"
    if path.exists() and not refresh:
        d=pd.read_csv(path)
        d=d[(d.time>=start_ms)&(d.time<=end_ms)]
        if len(d)>0: return d.sort_values("time").drop_duplicates("time").reset_index(drop=True)
    step=TF_MS[interval]; session=requests.Session(); rows=[]; cursor=end_ms+1; calls=0
    # The API caps a response at 200 candles, so walk backward by returned oldest timestamp.
    while cursor > start_ms:
        batch=api_get(KLINE,{"symbol":symbol,"startTime":start_ms-1,"endTime":cursor,"interval":interval,"limit":200,"type":"LAST_PRICE"},session).get("data",[])
        calls+=1
        if not batch: break
        parsed=[]
        for x in batch:
            try:
                t=int(x["time"])
                if start_ms<=t<=end_ms:
                    parsed.append({"time":t,"open":float(x["open"]),"high":float(x["high"]),"low":float(x["low"]),"close":float(x["close"]),"baseVol":float(x["baseVol"]),"quoteVol":float(x["quoteVol"])})
            except (KeyError,TypeError,ValueError): continue
        rows.extend(parsed)
        oldest=min((int(x["time"]) for x in batch),default=cursor)
        if oldest >= cursor or oldest <= start_ms or len(batch)<200: break
        cursor=oldest
        time.sleep(0.115)  # documented public IP rate limit: 10 requests/sec
    if not rows: raise APIError(f"No {symbol} {interval} candles returned for selected range")
    d=pd.DataFrame(rows).sort_values("time").drop_duplicates("time").reset_index(drop=True)
    d.to_csv(path,index=False)
    print(f"Fetched {symbol} {interval}: {len(d)} candles in {calls} requests")
    return d

def fetch_funding(symbol: str,start_ms: int,end_ms: int) -> pd.DataFrame:
    session=requests.Session(); rows=[]; cursor=end_ms+1
    while cursor>start_ms:
        data=api_get(FUNDING,{"symbol":symbol,"startTime":start_ms-1,"endTime":cursor,"limit":200},session).get("data",[])
        if not data: break
        rows.extend(data)
        oldest=min(int(x["fundingTime"]) for x in data)
        if oldest>=cursor or oldest<=start_ms or len(data)<200: break
        cursor=oldest; time.sleep(.115)
    if not rows: return pd.DataFrame(columns=["time","fundingRate","markPrice"])
    out=pd.DataFrame([{"time":int(x["fundingTime"]),"fundingRate":float(x["fundingRate"]),"markPrice":float(x["markPrice"])} for x in rows if start_ms<=int(x["fundingTime"])<=end_ms])
    return out.drop_duplicates("time").sort_values("time").reset_index(drop=True)

def indicators(df: pd.DataFrame) -> pd.DataFrame:
    d=df.copy(); c=d.close; h=d.high; l=d.low; o=d.open; v=d.baseVol
    # Conventional, explicitly chosen baseline parameters.
    ema9=c.ewm(span=9,adjust=False).mean(); ema21=c.ewm(span=21,adjust=False).mean()
    ma20=c.rolling(20).mean(); ma50=c.rolling(50).mean()
    delta=c.diff(); gain=delta.clip(lower=0); loss=-delta.clip(upper=0)
    avg_gain=gain.ewm(alpha=1/14,min_periods=14,adjust=False).mean(); avg_loss=loss.ewm(alpha=1/14,min_periods=14,adjust=False).mean()
    rs=avg_gain/avg_loss.replace(0,np.nan); rsi=100-(100/(1+rs))
    macd=c.ewm(span=12,adjust=False).mean()-c.ewm(span=26,adjust=False).mean(); macd_sig=macd.ewm(span=9,adjust=False).mean()
    vmean=v.rolling(20).mean(); roc=c/c.shift(10)-1
    tr=pd.concat([(h-l),(h-c.shift()).abs(),(l-c.shift()).abs()],axis=1).max(axis=1)
    atr=tr.ewm(alpha=1/14,min_periods=14,adjust=False).mean()
    prior_hi=h.shift(1).rolling(20).max(); prior_lo=l.shift(1).rolling(20).min()
    mid=c.rolling(20).mean(); sd=c.rolling(20).std(ddof=0); upper=mid+2*sd; lower=mid-2*sd
    votes=pd.DataFrame(index=d.index)
    votes["EMA"]=np.select([ema9>ema21,ema9<ema21],[1,-1],default=0)
    votes["MA"]=np.select([ma20>ma50,ma20<ma50],[1,-1],default=0)
    votes["RSI"]=np.select([rsi<30,rsi>70],[1,-1],default=0)
    votes["MACD"]=np.where(macd>macd_sig,1,-1)
    votes["VOLUME"]=np.select([(v>vmean)&(c>o),(v>vmean)&(c<o)],[1,-1],default=0)
    votes["MOMENTUM"]=np.select([roc>0,roc<0],[1,-1],default=0)
    votes["ATR_BREAKOUT"]=np.select([c>prior_hi,c<prior_lo],[1,-1],default=0)
    # Supertrend ATR(10) x 3, implemented as trailing bands.
    atr10=tr.ewm(alpha=1/10,min_periods=10,adjust=False).mean(); hl2=(h+l)/2
    basic_up=hl2+3*atr10; basic_dn=hl2-3*atr10
    final_up=basic_up.copy(); final_dn=basic_dn.copy(); st_dir=pd.Series(0,index=d.index,dtype=int)
    for i in range(1,len(d)):
        if pd.isna(basic_up.iloc[i]) or pd.isna(basic_dn.iloc[i]): continue
        final_up.iloc[i]=basic_up.iloc[i] if (basic_up.iloc[i]<final_up.iloc[i-1] or c.iloc[i-1]>final_up.iloc[i-1]) else final_up.iloc[i-1]
        final_dn.iloc[i]=basic_dn.iloc[i] if (basic_dn.iloc[i]>final_dn.iloc[i-1] or c.iloc[i-1]<final_dn.iloc[i-1]) else final_dn.iloc[i-1]
        prev=st_dir.iloc[i-1]
        if prev<=0: st_dir.iloc[i]=1 if c.iloc[i]>final_up.iloc[i] else -1
        else: st_dir.iloc[i]=-1 if c.iloc[i]<final_dn.iloc[i] else 1
    votes["SUPERTREND"]=st_dir
    votes["BOLLINGER"]=np.select([c<lower,c>upper],[1,-1],default=0)
    votes=votes.replace([np.inf,-np.inf],np.nan).fillna(0).astype(int)
    d["availableTime"]=d.time+TF_MS[str(df.attrs.get("interval","15m"))]
    for name in STRATEGIES: d[f"v_{name}"]=votes[name].values
    return d

def frame_signal(df: pd.DataFrame) -> pd.DataFrame:
    v=df[[f"v_{x}" for x in STRATEGIES]]
    long=(v==1).sum(axis=1); short=(v==-1).sum(axis=1); active=long+short
    winner=np.where(long>short,1,np.where(short>long,-1,0))
    winning=np.maximum(long,short)
    conf=np.where(active>0,100*winning/active,0.0)
    valid=(winning>=3)&(conf>=60)&(winner!=0)
    return pd.DataFrame({"availableTime":df.availableTime,"tf_direction":np.where(valid,winner,0),"tf_confidence":np.where(valid,conf,0),"long_votes":long,"short_votes":short,"active_votes":active})

def combine_timeframes(frames: dict[str,pd.DataFrame], common_start: int, selected_tfs: list[str]) -> pd.DataFrame:
    # Execute on 15m bars. Each signal is available only after its source candle closes.
    base=frames["15m"].loc[frames["15m"].time>=common_start].copy().sort_values("time")
    base["signalTime"]=base.time+TF_MS["15m"]
    result=base[["time","signalTime","open","high","low","close"]].copy()
    tf_sigs={tf:frame_signal(frames[tf]).sort_values("availableTime") for tf in selected_tfs}
    for tf, sig in tf_sigs.items():
        right=sig.rename(columns={c:f"{tf}_{c}" for c in sig.columns if c!="availableTime"})
        result=pd.merge_asof(result.sort_values("signalTime"),right.sort_values("availableTime"),left_on="signalTime",right_on="availableTime",direction="backward")
        result=result.drop(columns=["availableTime"])
    dirs=result[[f"{tf}_tf_direction" for tf in selected_tfs]].fillna(0).astype(int)
    aligned_long=(dirs==1).sum(axis=1); aligned_short=(dirs==-1).sum(axis=1)
    # Min confidence=80% means at least 80% of the selected timeframes must align.
    tf_count=len(selected_tfs); win=np.maximum(aligned_long,aligned_short); conf=100*win/tf_count
    direction=np.where(aligned_long>aligned_short,1,np.where(aligned_short>aligned_long,-1,0))
    result["direction"]=np.where((conf>=80)&(direction!=0),direction,0)
    result["confidence_pct"]=conf
    result["aligned_timeframes"]=win
    return result

def backtest(signals: pd.DataFrame,funding: pd.DataFrame,capital=10000.0,position_fraction=1.0):
    # Single position, 1x notional, no trading fee/slippage. Enter at next 15m open.
    d=signals.reset_index(drop=True); cash=float(capital); pos=None; trades=[]; eq=[]; funding_total=0.0
    fund=funding.sort_values("time").to_dict("records") if len(funding) else []
    for i in range(1,len(d)):
        prior=d.iloc[i-1]; bar=d.iloc[i]; ts=int(bar.time)
        desired=int(prior.direction)
        if pos and desired!=pos["direction"] and desired!=0:
            px=float(bar.open); pnl=pos["direction"]*(px-pos["entry"])*pos["qty"]+pos["funding"]
            cash+=pnl; trades.append({**pos,"exit_time":ts,"exit_price":px,"pnl":pnl,"funding_pnl":pos["funding"],"return_pct":100*pnl/pos["capital_at_entry"]});pos=None
        if pos and desired==0:
            # Neutral does not force exit; keep position until explicit opposite signal or end.
            pass
        if pos is None and desired!=0:
            px=float(bar.open); alloc=max(0.0,cash*position_fraction); qty=alloc/px
            pos={"direction":desired,"entry_time":ts,"entry":px,"qty":qty,"capital_at_entry":cash,"funding":0.0}
        if pos:
            # Funding settles at supplied timestamps while position is open. Long pays positive rates.
            for f in fund:
                if pos["entry_time"]<int(f["time"])<=ts and not f.get("used",False):
                    fp=-pos["direction"]*pos["qty"]*float(f["markPrice"])*float(f["fundingRate"])
                    pos["funding"]+=fp; funding_total+=fp; f["used"]=True
            unreal=pos["direction"]*(float(bar.close)-pos["entry"])*pos["qty"]+pos["funding"]
            eq.append({"time":ts,"equity":cash+unreal,"close":float(bar.close),"signal":desired,"confidence_pct":float(prior.confidence_pct)})
        else: eq.append({"time":ts,"equity":cash,"close":float(bar.close),"signal":desired,"confidence_pct":float(prior.confidence_pct)})
    if pos:
        last=d.iloc[-1]; px=float(last.close); pnl=pos["direction"]*(px-pos["entry"])*pos["qty"]+pos["funding"]
        cash+=pnl; trades.append({**pos,"exit_time":int(last.time)+TF_MS["15m"],"exit_price":px,"pnl":pnl,"funding_pnl":pos["funding"],"return_pct":100*pnl/pos["capital_at_entry"]})
        if eq: eq[-1]["equity"]=cash
    eqdf=pd.DataFrame(eq)
    if eqdf.empty: raise ValueError("Not enough 15m candles for backtest")
    start=float(capital); final=float(eqdf.equity.iloc[-1]); rets=eqdf.equity.pct_change().replace([np.inf,-np.inf],np.nan).dropna()
    dd=eqdf.equity/eqdf.equity.cummax()-1
    sharpe=float(rets.mean()/rets.std(ddof=1)*math.sqrt(365*24*4)) if len(rets)>1 and rets.std(ddof=1)>0 else 0.0
    downside=rets[rets<0]; sortino=float(rets.mean()/downside.std(ddof=1)*math.sqrt(365*24*4)) if len(downside)>1 and downside.std(ddof=1)>0 else 0.0
    days=max(1,(int(eqdf.time.iloc[-1])-int(eqdf.time.iloc[0]))/86_400_000)
    cagr=((final/start)**(365.25/days)-1)*100 if final>0 else -100.0
    wins=sum(1 for t in trades if t["pnl"]>0); losses=sum(1 for t in trades if t["pnl"]<0)
    grosswin=sum(t["pnl"] for t in trades if t["pnl"]>0); grossloss=abs(sum(t["pnl"] for t in trades if t["pnl"]<0))
    metrics={"initial_equity":start,"final_equity":final,"total_return_pct":100*(final/start-1),"cagr_pct":cagr,"max_drawdown_pct":100*float(dd.min()),"sharpe_15m_annualized_365d":sharpe,"sortino_15m_annualized_365d":sortino,"trade_count":len(trades),"win_rate_pct":100*wins/len(trades) if trades else 0.0,"profit_factor":grosswin/grossloss if grossloss else (float("inf") if grosswin else 0.0),"funding_pnl_usdt":funding_total,"trading_fees_modeled_usdt":0.0,"slippage_modeled_bps":0.0,"leverage":1.0,"liquidation_simulated":False}
    return metrics,pd.DataFrame(trades),eqdf

def main():
    p=argparse.ArgumentParser(description=__doc__)
    p.add_argument('--symbol',default='BTCUSDT');p.add_argument('--days',type=int,default=30);p.add_argument('--capital',type=float,default=10000)
    p.add_argument('--position-fraction',type=float,default=1.0);p.add_argument('--timeframes',default='1m,3m,5m,15m',help='Comma-separated signal intervals; execution always uses 15m');p.add_argument('--refresh',action='store_true');args=p.parse_args()
    selected_tfs=[x.strip() for x in args.timeframes.split(',') if x.strip()]
    if args.days<1 or not 0<args.position_fraction<=1: raise SystemExit('days must be positive and position-fraction in (0,1]')
    if not selected_tfs or any(x not in TF_MS for x in selected_tfs) or '15m' not in selected_tfs: raise SystemExit('timeframes must use 1m,3m,5m,15m and include 15m execution')
    end=int(time.time()*1000); start=end-args.days*86_400_000
    start=(start//TF_MS['1m'])*TF_MS['1m']; end=(end//TF_MS['15m'])*TF_MS['15m']-1
    print(f"Bitunix public futures LAST_PRICE candles: {args.symbol}; {datetime.fromtimestamp(start/1000,timezone.utc).isoformat()} to {datetime.fromtimestamp(end/1000,timezone.utc).isoformat()}; 1m/3m/5m/15m")
    raw={tf:fetch_klines(args.symbol,tf,start,end,args.refresh) for tf in selected_tfs}
    common_start=max(int(df.time.min()) for df in raw.values())
    common_end=min(int(df.time.max()) for df in raw.values())
    if common_end<=common_start: raise APIError('Timeframes have no overlapping candle history')
    frames={}
    for tf,df in raw.items():
        df.attrs['interval']=tf; frames[tf]=indicators(df)
    sig=combine_timeframes(frames,common_start,selected_tfs)
    funding=fetch_funding(args.symbol,common_start,common_end)
    print(f"Common all-timeframe coverage: {datetime.fromtimestamp(common_start/1000,timezone.utc).isoformat()} to {datetime.fromtimestamp(common_end/1000,timezone.utc).isoformat()}")
    print(f"Historical funding settlements: {len(funding)}")
    metrics,trades,equity=backtest(sig,funding,args.capital,args.position_fraction)
    actual_days=(common_end-common_start)/86_400_000
    if actual_days<90:
        metrics['cagr_pct']=None;metrics['sharpe_15m_annualized_365d']=None;metrics['sortino_15m_annualized_365d']=None
        metrics['annualized_metrics_note']='Omitted: less than 90 days of common multi-timeframe history; annualization would be misleading.'
    OUT_DIR.mkdir(parents=True,exist_ok=True); stamp=datetime.now(timezone.utc).strftime('%Y%m%dT%H%M%SZ'); prefix=f"{args.symbol}_{args.days}d_{stamp}"
    for tf,df in raw.items(): df.to_csv(OUT_DIR/f'{prefix}_{tf}_candles.csv',index=False)
    trades.to_csv(OUT_DIR/f'{prefix}_trades.csv',index=False);equity.to_csv(OUT_DIR/f'{prefix}_equity.csv',index=False)
    metrics['symbol']=args.symbol;metrics['requested_lookback_days']=args.days;metrics['actual_common_start_utc']=datetime.fromtimestamp(common_start/1000,timezone.utc).isoformat();metrics['actual_common_end_utc']=datetime.fromtimestamp(common_end/1000,timezone.utc).isoformat();metrics['actual_common_days']=(common_end-common_start)/86_400_000;metrics['candles_by_timeframe']={tf:len(df[(df.time>=common_start)&(df.time<=common_end)]) for tf,df in raw.items()};metrics['execution_timeframe']='15m';metrics['signal_timeframes']=selected_tfs;metrics['signal_rules']={'indicators':STRATEGIES,'parameters':'EMA 9/21; SMA 20/50; RSI 14 (30/70); MACD 12/26/9; volume vs SMA20 with candle direction; ROC10; prior 20-bar ATR breakout; Supertrend ATR10 x3; Bollinger20 x2','per_timeframe':'at least 3 same-direction indicator votes and >=60% agreement among non-neutral votes','global':f'at least 80% of selected timeframe votes; selected={selected_tfs}','execution':'signal after candle close, entry at next 15m open; exit on opposite qualified signal; final close at last close'}
    with open(OUT_DIR/f'{prefix}_metrics.json','w') as f: json.dump(metrics,f,indent=2,allow_nan=True)
    report=OUT_DIR/f'{prefix}_report.md'
    lines=[f"# Bitunix futures backtest — {args.symbol}","",f"Requested: {args.days} days. Actual common history across selected intervals: {datetime.fromtimestamp(common_start/1000,timezone.utc).isoformat()} to {datetime.fromtimestamp(common_end/1000,timezone.utc).isoformat()} UTC ({(common_end-common_start)/86_400_000:.2f} days). Candles: LAST_PRICE | base execution: 15m | signal frames: {", ".join(selected_tfs)}","", "## Assumptions", "- Uses Bitunix public futures candle and historical funding-rate endpoints; no private credentials or orders.","- Signals: EMA 9/21; SMA 20/50; RSI 14 at 30/70; MACD 12/26/9; volume above SMA20 plus candle direction; 10-bar momentum; prior 20-bar high/low breakout; Supertrend ATR10 x3; Bollinger20 x2 (contrarian outside-band votes).","- A timeframe qualifies with at least 3 aligned indicator votes and >=60% of its non-neutral votes aligned. Overall confidence is aligned selected timeframes divided by the selected timeframe count; the requested >=80% is applied to that ratio. In a single-timeframe run, this does not validate multi-timeframe confluence. This is an explicit interpretation, not an exchange-defined confidence score.","- Decisions use closed bars only; a 15m signal enters at the next 15m open. One position at a time; position sizing uses 1x notional up to the specified equity fraction. Reverse qualified signal exits/reverses; otherwise position stays open.","- Historical funding rates are applied at settlement timestamps using reported mark price; positive funding is paid by longs and received by shorts. Trading fees are modeled as 0 bps and slippage as 0 bps because the account fee tier and fill data were not provided. Results are therefore pre-fee and idealized. No leverage above 1x, liquidation, spread, partial fills, stop-loss/TP, or exchange contract precision is simulated.","", "## Results"]
    for k,v in metrics.items(): lines.append(f"- **{k}:** {v}")
    lines += ["",f"Trades: {len(trades)} | funding settlements fetched: {len(funding)} | candle files, equity, trades, and metrics saved alongside this report.","", "**Research only; not financial advice or a live-trading validation.**"]
    report.write_text('\n'.join(lines)+'\n')
    print('\n'.join(lines[:24]));print(f"\nReport: {report}")
if __name__=='__main__': main()
