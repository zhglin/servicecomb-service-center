/*
 * Licensed to the Apache Software Foundation (ASF) under one or more
 * contributor license agreements.  See the NOTICE file distributed with
 * this work for additional information regarding copyright ownership.
 * The ASF licenses this file to You under the Apache License, Version 2.0
 * (the "License"); you may not use this file except in compliance with
 * the License.  You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import {Component, Input, OnChanges, OnInit, SimpleChanges, ViewChild} from '@angular/core';
import {MatTableDataSource} from '@angular/material/table';
import {SelectionModel} from '@angular/cdk/collections';
import {MatPaginator} from '@angular/material/paginator';
import {Observable} from 'rxjs';
import {InstanceModel} from '../../../../models/instance.model';


@Component({
  selector: 'app-instances-table',
  templateUrl: './instances-table.component.html',
  styleUrls: ['./instances-table.component.scss']
})
export class InstancesTableComponent implements OnInit, OnChanges {
  @Input() instancesTableData$: Observable<InstanceModel[]>;
  public displayedColumns: string[] = ['select', 'id', 'serviceName', 'host', 'endPoint', 'kieConf', 'status'];
  public dataSource: MatTableDataSource<InstanceModel> = new MatTableDataSource<InstanceModel>([]);
  public selection = new SelectionModel<InstanceModel>(true, []);

  public isShowFilterInput = false;

  @ViewChild(MatPaginator, {static: true}) paginator: MatPaginator;

  public ngOnInit(): void {
    this.instancesTableData$.subscribe(d => {
      this.dataSource = new MatTableDataSource<InstanceModel>(d);
      this.dataSource.paginator = this.paginator;
    });

  }

  public ngOnChanges(changes: SimpleChanges): void {
  }

  /** Whether the number of selected elements matches the total number of rows. */
  public isAllSelected(): boolean {
    const numSelected = this.selection.selected.length;
    const numRows = this.dataSource.data.length;
    return numSelected === numRows;
  }

  /** Selects all rows if they are not all selected; otherwise clear selection. */
  public masterToggle(): void {
    this.isAllSelected() ?
      this.selection.clear() :
      this.dataSource.data.forEach(row => this.selection.select(row));
  }

  /** The label for the checkbox on the passed row */
  public checkboxLabel(row?: any): string {
    if (!row) {
      return `${this.isAllSelected() ? 'select' : 'deselect'} all`;
    }
    return `${this.selection.isSelected(row) ? 'deselect' : 'select'} row ${row.position + 1}`;
  }

  public applyFilter(event: Event): void {
    const filterValue = (event.target as HTMLInputElement).value;
    this.dataSource.filter = filterValue.trim().toLowerCase();
  }

  public showFilterInput(): void {
    this.isShowFilterInput = !this.isShowFilterInput;
    //this.dataSource = new MatTableDataSource<ServiceModel>(this.servicesTableData);
  }
}
