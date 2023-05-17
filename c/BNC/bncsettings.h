#ifndef BNCSETTINGS_H
#define BNCSETTINGS_H

#include <QMutex>
#include <QVariant>

class bncSettings {
 public:
  bncSettings();
  ~bncSettings();
  QVariant value(const QString& key,
                 const QVariant& defaultValue = QVariant()) const;
  void setValue(const QString &key, const QVariant& value);
  void remove(const QString& key );
  bool contains(const QString& key) const;
  void reRead();
  void sync();
 private:
  void setValue_p(const QString &key, const QVariant& value);
  static QMutex _mutex;
};

#endif
